/*
Copyright 2022 Natun.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/natun-ai/natun/api"
	"github.com/natun-ai/natun/internal/plugins/providers/historical/parquet"
	"github.com/natun-ai/natun/pkg/plugins"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/xitongsys/parquet-go-source/s3v2"
	"github.com/xitongsys/parquet-go/source"
	"time"
)

const pluginName = "parquet-aws"

func init() {
	plugins.Configurers.Register(pluginName, BindConfig)
	plugins.HistoricalWriterFactories.Register(pluginName, HistoricalWriterFactory)
}

func BindConfig(set *pflag.FlagSet) error {
	set.String("aws-access-key", "", "AWS Access Key - for historical data")
	set.String("aws-secret-key", "", "AWS Secret Key - for historical data")
	set.String("aws-region", "", "AWS Region - for historical data")
	set.String("s3-bucket", "", "S3 Bucket - for historical data")
	set.String("s3-basedir", "natun/features/", "S3 Base directory for storing features - for historical data")
	return nil
}

func HistoricalWriterFactory(viper *viper.Viper) (api.HistoricalWriter, error) {
	var opts []func(*config.LoadOptions) error
	if viper.GetString("aws-access-key") != "" && viper.GetString("aws-secret-key") != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     viper.GetString("aws-access-key"),
				SecretAccessKey: viper.GetString("aws-secret-key"),
			},
		}))
	}
	if viper.GetString("aws-region") != "" {
		opts = append(opts, config.WithRegion(viper.GetString("aws-region")))
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}
	client := s3.NewFromConfig(cfg)

	bucket := viper.GetString("s3-bucket")
	if bucket == "" {
		return nil, fmt.Errorf("s3-bucket is required")
	}
	_, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check s3 bucket: %w", err)
	}

	factory := sourceFactory(client, bucket, viper.GetString("s3-basedir"))

	return parquet.BaseParquet(4, factory), nil
}
func sourceFactory(client s3v2.S3API, bucket string, basedir string) parquet.SourceFactory {
	return func(ctx context.Context, fqn string, alive bool) (source.ParquetFile, error) {
		if basedir[len(basedir)-1] != '/' {
			basedir += "/"
		}
		d := time.Now().Format("2006-01-02")
		aliveTag := ""
		if alive {
			aliveTag = "-alive"
		}
		filename := fmt.Sprintf("%sfqn=%s/timestamp=%s/data%s.snappy.parquet", basedir, fqn, d, aliveTag)
		return s3v2.NewS3FileWriterWithClient(ctx, client, bucket, filename, nil)
	}
}
