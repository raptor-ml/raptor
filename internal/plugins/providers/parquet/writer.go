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

package parquet

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/xitongsys/parquet-go/source"
	"github.com/xitongsys/parquet-go/writer"
	"sync"
)

type SourceFactory func(ctx context.Context, fqn string) (source.ParquetFile, error)
type baseParquet struct {
	newParquetFile SourceFactory
	np             int64
	writers        map[string]*parquetWriter
}

func BaseParquet(np int64, newParquetFile SourceFactory) *baseParquet {
	return &baseParquet{
		newParquetFile: newParquetFile,
		np:             np,
		writers:        make(map[string]*parquetWriter),
	}
}

type parquetWriter struct {
	*writer.ParquetWriter
	*sync.Mutex
}

func (bw *baseParquet) Commit(ctx context.Context, wn api.WriteNotification) error {
	pw, err := bw.getWriter(ctx, wn.FQN)
	if err != nil {
		return err
	}
	pw.Lock()
	defer pw.Unlock()
	return pw.Write(NewHistoricalRecord(wn))
}

func (bw *baseParquet) getWriter(ctx context.Context, fqn string) (*parquetWriter, error) {
	if _, ok := bw.writers[fqn]; !ok {
		pf, err := bw.newParquetFile(ctx, fqn)
		if err != nil {
			return nil, fmt.Errorf("cannot create parquet file: %w", err)
		}
		pw, err := writer.NewParquetWriter(pf, new(HistoricalRecord), bw.np)
		if err != nil {
			return nil, fmt.Errorf("cannot create parquet writer: %w", err)
		}
		pw.PageSize = 1 * 1024 * 1024       // 100M
		pw.RowGroupSize = 256 * 1024 * 1024 // 256M
		createdBy := "natun-historian version latest"
		pw.Footer.CreatedBy = &createdBy
		bw.writers[fqn] = &parquetWriter{
			ParquetWriter: pw,
			Mutex:         &sync.Mutex{},
		}
	}
	return bw.writers[fqn], nil
}
func (bw *baseParquet) Flush(ctx context.Context, fqn string) error {
	pw, err := bw.getWriter(ctx, fqn)
	if err != nil {
		return err
	}
	pw.Lock()
	defer pw.Unlock()

	err = pw.WriteStop()
	if err != nil {
		return fmt.Errorf("cannot write stop: %w", err)
	}
	err = pw.PFile.Close()
	if err != nil {
		return fmt.Errorf("cannot close parquet file: %w", err)
	}
	delete(bw.writers, fqn)
	return nil
}

func (bw *baseParquet) FlushAll(ctx context.Context) error {
	for fqn := range bw.writers {
		err := bw.Flush(ctx, fqn)
		if err != nil {
			return err
		}
	}
	return nil
}
