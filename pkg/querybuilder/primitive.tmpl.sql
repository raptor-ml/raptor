{{- /*gotype: github.com/raptor-ml/raptor/internal/querybuilder.FeatureSetQueryData */ -}}
WITH results as (
    SELECT  FQN,
            ENTITY_ID,
            TIMESTAMP,
            VALUE,
            {{- /* Add expiration of this value */}}
            LAG(TIMESTAMP, 1) OVER (partition by FQN, ENTITY_ID ORDER BY TIMESTAMP DESC) AS _NEXT_TIMESTAMP,
            {{subtractDuration .Staleness "TIMESTAMP"}}                                  AS _EXPIRE,
            CASE
                WHEN _NEXT_TIMESTAMP < _EXPIRE THEN _NEXT_TIMESTAMP
                ELSE _EXPIRE END                                                         AS VALID_TILL
    FROM {{.FeaturesTable}}
    WHERE FQN = '{{.FQN}}'
        AND TIMESTAMP BETWEEN {{.Since}} AND {{.Until}}
        AND BUCKET IS NULL
    ORDER BY FQN, TIMESTAMP, ENTITY_ID
)
SELECT FQN,
       ENTITY_ID,
       TIMESTAMP,
       VALUE,
       VALID_TILL
FROM results;