{{- /* gotype: github.com/raptor-ml/raptor/pkg/querybuilder.featureSetQuery */ -}}
{{- /* @formatter:off */ -}}
{{- /***
  # Point in time join query
  --------------------------
  To achieve that we create a CTE per feature and join the key feature with each CTE

  ## Requirements:
   - A key feature or set of times to align to

  1. Select all the data
    1.1. base - base data is selecting since $SINCE minus the biggest window size
            (to allow windows that started exactly in $SINCE to be included)
    1.2. data - the base without the "extra time" - we'll use that for our joins
    1.3. primitivesData - the data without the windowed features
  2. Prepare the Windows data - for each window create the following CTEs:
    2.1. winData_f_XX - the windowed data for the feature
    2.2. f_XX - the windowed feature
        2.2.1. Joining winDataXX with itself in the range of the window
        2.2.2. Take only the latest value*
                ORDER BY feature.TIMESTAMP DESC LIMIT 1
  3. Prepare the primitives' data - for each primitive create the `f_XX` CTE
    3.1. WHERE fqn=<fqn>
    3.2. ORDER BY feature.TIMESTAMP DESC LIMIT 1*
  4. Build the final view by join the key feature with each feature CTE
    4.1. ON f_XX.KEYS = keyFeature.KEYS
         AND f_XX.TIMESTAMP <= keyFeature.TIMESTAMP
         AND f_XX.timestamp >= DATEADD(<staleness_unit>, <-staleness>, f_XX.timestamp)

  * - if the feature is the key feature, don't limit the number of rows
 ***/ -}}
WITH
    {{- /* 1. Get all the data relevant for this feature set */}}
    base AS (
        SELECT  FQN,
                KEYS,
                TIMESTAMP,
                VALUE AS VAL,
                BUCKET,
                BUCKET_ACTIVE
        FROM {{.FeaturesTable}}
        {{- /* we should take a greater time before $SINCE to avoid a window that started exactly at 00:00 */}}
        WHERE TIMESTAMP BETWEEN {{subtractDuration .BeforePadding .Since}}
            AND {{.Until}}
            AND FQN IN (
            {{- range $i, $f := .Features -}}
                {{- if ne $i 0}}, {{end -}}
                '{{$f.FQN}}'
            {{- end -}})
    ),
    data as (SELECT * FROM base WHERE TIMESTAMP >= {{.Since}}),
    primitivesData AS (SELECT * FROM data WHERE BUCKET IS NULL)
{{- range $_, $f := .Features}}
{{- if $f.ValidWindow}}
    {{- /* 2.1. Get the buckets data with start and end dates */ -}}
    ,
    winData_{{tmpName $f.FQN}} AS (
        SELECT *, {{subtractDuration $f.Staleness "timestamp"}} AS WIN_START, TIMESTAMP AS WIN_END
        FROM base
        WHERE BUCKET IS NOT NULL
          AND BUCKET_ACTIVE = false
          AND FQN = '{{$f.FQN}}'
          AND TIMESTAMP >= {{subtractDuration $f.Staleness $.Since}}
    ),
    {{- /* 2.2. Get the windowed feature */}}
    {{tmpName $f.FQN}} AS (
        SELECT
            b1.KEYS,
            b1.WIN_START,
            b1.WIN_END,
            b1.WIN_END as TIMESTAMP,
            sum (b1.VAL['count']) as _ COUNT,
            sum (b1.VAL['sum']) as _ SUM,
            min (b1.VAL['min']) as _ MIN,
            max (b1.VAL['max']) as _ MAX,
            (_ SUM / _ COUNT) as _ AVG,
            OBJECT_CONSTRUCT( {{- /*- building a unified value object*/}}
                'count',
                _ COUNT :: int ::variant, 'sum',
                _ SUM :: int ::variant,
                'min', _ MIN :: int ::variant,
                'max', _ MAX :: int ::variant,
                'avg', _ AVG :: double ::variant
            ) as VAL
        FROM (
            SELECT * FROM winData_{{tmpName $f.FQN}} WHERE WIN_END >= {{$.Since}}) b1
                LEFT JOIN winData_{{tmpName $f.FQN}} AS b2 ON b1.KEYS = b2.KEYS
                    AND b2.WIN_END >= b1.WIN_START AND b2.WIN_END < b1.WIN_END AND b2.BUCKET != b1.BUCKET
            GROUP BY b1.KEYS, b1.WIN_START, b1.WIN_END
            {{ if ne $f.FQN $.KeyFeature}}ORDER BY TIMESTAMP DESC LIMIT 1{{end}}
    )
{{- else}}
    {{- /* 3. Get the buckets data with start and end dates */ -}}
    ,
    {{tmpName $f.FQN}} AS (
        SELECT *
        FROM primitivesData
        WHERE FQN = '{{$f.FQN}}'
            {{ if ne $f.FQN $.KeyFeature}}ORDER BY TIMESTAMP DESC LIMIT 1{{end}}
    )
{{- end}}
{{- end}}
{{- /* 4. Build the final results */}}
SELECT  key.TIMESTAMP,
        key.KEYS
{{- range $_, $f := .Features}},
    {{- if eq $f.FQN $.KeyFeature}}
        key.VAL as {{escapeName $f.FQN}}
    {{- else}}
        {{printf "%s.VAL" (tmpName $f.FQN)}} as {{escapeName $f.FQN}}
    {{- end}}
{{- end}}
    FROM {{tmpName .KeyFeature}} as key
{{- range $_, $f := .Features}}
{{- if eq $f.FQN $.KeyFeature}}{{continue}}{{end}}
{{- $n := tmpName $f.FQN}}
    {{- /* 4.1. Join the KeyFeature with the feature's CTE */}}
        LEFT JOIN {{$n}}
        ON {{$n}}.KEYS = key.KEYS AND {{$n}}.TIMESTAMP <= key.TIMESTAMP
                    AND {{$n}}.TIMESTAMP >= {{subtractDuration $f.Staleness "key.TIMESTAMP"}}
{{- end}}
    ORDER BY TIMESTAMP;
