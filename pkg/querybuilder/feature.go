/*
 * Copyright (c) 2022 Raptor.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package querybuilder

import (
	"bytes"
	"fmt"
	"github.com/raptor-ml/raptor/api"
	"text/template"
)

func (qb *queryBuilder) Feature(ft api.Metadata) (string, error) {
	data := featureQuery{
		baseQuery: baseQuery{
			FeaturesTable: qb.featureTable,
			Since:         "$SINCE",
			Until:         "$UNTIL",
		},
		Metadata: ft,
	}

	var tpl *template.Template
	if ft.ValidWindow() {
		tpl = qb.tpls.Lookup("windowed.tmpl.sql")
	} else {
		tpl = qb.tpls.Lookup("primitive.tmpl.sql")
	}

	var query bytes.Buffer
	err := tpl.Execute(&query, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return query.String(), nil
}
