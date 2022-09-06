{{- /*gotype: github.com/raptor-ml/raptor/internal/querybuilder.featureQuery */ -}}
    {{- /***
      # Windowed feature
      --------------------------
      To achieve that we create a CTE of the feature's buckets with it's start and end timestamps,
      and join it with itself with all the other buckets that match this time frame.

      1. Select all the data
        1.1. base - base data is selecting since $SINCE minus the window size
                (to allow windows that started exactly in $MIN_DATE to be included)
        1.2. data - the base without the "extra time" - we'll use that for our joins
      2. Prepare the Windows data - for each window create the following CTEs:
        2.1. winData_f_XX - the windowed data for the feature
        2.2. f_XX - the windowed feature
            2.2.1. Joining winDataXX with itself in the range of the window
            2.2.2. Take only the latest value for each entity_id
      3. Show the result ordered
     ***/ -}}
    WITH
        {{- /* 1. Get all the data relevant for this feature set */}} base AS (SELECT FQN,
                                                                                      ENTITY_ID,
        TIMESTAMP
       , VALUE AS VAL
       , BUCKET
       , BUCKET_ACTIVE
    FROM {{.FeaturesTable}}
        {{- /* we should take a greater time before $SINCE to avoid a window that started exactly at 00:00 */}}
    WHERE TIMESTAMP BETWEEN {{subtractDuration .Staleness .Since}}
      AND {{.Until}}
      AND FQN = '{{.FQN}}'
        )
        , data as (
    SELECT *
    FROM base
    WHERE TIMESTAMP >= {{.Since}})
        , {{- /* 2.1. Get the buckets data with start and end dates */}}
        winData_{{tmpName .FQN}} AS (
    SELECT *, {{subtractDuration .Staleness "timestamp"}} AS WIN_START, TIMESTAMP AS WIN_END
    FROM base
    WHERE BUCKET IS NOT NULL
      AND BUCKET_ACTIVE = false
      AND FQN = '{{.FQN}}'
      AND TIMESTAMP >= {{subtractDuration .Staleness $.Since}}
        )
        , {{- /* 2.2. Get the windowed feature */}}
        {{tmpName .FQN}} AS (
    SELECT
        b1.FQN,
        b1.ENTITY_ID,
        b1.WIN_START,
        b1.WIN_END,
        b1.WIN_END as TIMESTAMP,
        sum (b1.VAL['count']) as _ COUNT,
        sum (b1.VAL['sum']) as _ SUM,
        min (b1.VAL['min']) as _ MIN,
        max (b1.VAL['max']) as _ MAX,
        (_ SUM / _ COUNT) as _ AVG,
        OBJECT_CONSTRUCT({{- /*- building a unified value object*/}}
        'count', _ COUNT :: int ::variant,
        'sum', _ SUM :: int ::variant,
        'min', _ MIN :: int ::variant,
        'max', _ MAX :: int ::variant,
        'avg', _ AVG :: double ::variant
        ) as VAL
    FROM (SELECT * FROM winData_{{tmpName .FQN}} WHERE WIN_END >= {{$.Since}}) b1
        LEFT JOIN winData_{{tmpName .FQN}} as b2
    ON b1.ENTITY_ID = b2.ENTITY_ID
        AND b2.WIN_END >= b1.WIN_START and b2.WIN_END < b1.WIN_END AND b2.BUCKET != b1.BUCKET
    GROUP BY b1.FQN, b1.ENTITY_ID, b1.WIN_START, b1.WIN_END
        ),
        {{- /* Add expiration of this value */}}
        results as (
    SELECT *,
        LAG(TIMESTAMP, 1) OVER (partition by FQN, ENTITY_ID ORDER BY TIMESTAMP DESC) AS _NEXT_TIMESTAMP,
        {{subtractDuration .Staleness "TIMESTAMP"}} AS _EXPIRE,
        CASE
        WHEN _NEXT_TIMESTAMP < _EXPIRE THEN _NEXT_TIMESTAMP
        ELSE _EXPIRE END AS VALID_TILL
    FROM {{tmpName .FQN}}
        )
    SELECT FQN, ENTITY_ID, TIMESTAMP, VAL as VALUE, VALID_TILL
    FROM results
    ORDER BY FQN, ENTITY_ID, TIMESTAMP
