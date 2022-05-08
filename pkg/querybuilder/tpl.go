/*
 * Copyright (c) 2022 Natun.
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
	"context"
	"embed"
	"fmt"
	"github.com/natun-ai/natun/api"
	manifests "github.com/natun-ai/natun/api/v1alpha1"
	"strings"
	"text/template"
	"time"
)

//go:embed *.tmpl.sql
var tplFiles embed.FS

type baseQuery struct {
	FeaturesTable string
	Since         string
	Until         string
}
type featureQuery struct {
	baseQuery
	api.Metadata
}

type QueryBuilder interface {
	FeatureSet(ctx context.Context, fs manifests.FeatureSetSpec, getter api.MetadataGetter) (query string, err error)
	Feature(feature api.Metadata) (string, error)
}

type queryBuilder struct {
	tpls         *template.Template
	featureTable string
}

type Config struct {
	FeaturesTable string

	// EscapeName is used to escape the feature's FQN.
	EscapeName func(s string) string
	// SubtractDuration is used to subtract a duration from a field with your SQL flavor.
	SubtractDuration func(d time.Duration, field string) string
	// CastFeature is used to cast a feature to a specific type for your SQL flavor.
	CastFeature func(ft api.Metadata) string
	// TmpName is used to generate a temporary table name.
	TmpName func(s string) string
}

func New(config Config) QueryBuilder {
	if config.FeaturesTable == "" {
		panic("features table is required")
	}
	if config.SubtractDuration == nil {
		panic("subtract duration is required")
	}
	if config.CastFeature == nil {
		panic("cast feature is required")
	}
	if config.TmpName == nil {
		config.TmpName = tmpName
	}
	if config.EscapeName == nil {
		config.EscapeName = EscapeName
	}
	tpls := template.New("").Funcs(template.FuncMap{
		"escapeName":       config.EscapeName,
		"subtractDuration": config.SubtractDuration,
		"castFeature":      config.CastFeature,
		"tmpName":          config.TmpName,
	})
	tpls = template.Must(tpls.ParseFS(tplFiles, "*.sql"))

	return &queryBuilder{
		featureTable: config.FeaturesTable,
		tpls:         tpls,
	}
}

func tmpName(s string) string {
	return fmt.Sprintf("f_%s", strings.NewReplacer("-", "_", ".", "__").Replace(s))
}

func EscapeName(s string) string {
	return strings.NewReplacer("-", "_", ".", "$").Replace(s)
}
