/*
Copyright (c) 2022 RaptorML authors.

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

package protoregistry

import (
	"errors"
	"fmt"
	"github.com/die-net/lrucache"
	"github.com/google/uuid"
	"github.com/gregjones/httpcache"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// ErrAlreadyRegistered is returned when a descriptor is already registered.
var ErrAlreadyRegistered = fmt.Errorf("proto schema already registered")

// ErrNotFound is returned when a descriptor is not found.
var ErrNotFound = fmt.Errorf("not found")

// GetDescriptor returns a Protocol Buffer Descriptor given a fully qualified name (pacakge.MessageName).
func GetDescriptor(protoFqn string) (protoreflect.MessageDescriptor, error) {
	fd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(protoFqn))
	if err != nil {
		if errors.Is(err, protoregistry.NotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find a protobuf type `%s`: %w", protoFqn, err)
	}
	protoFQN := strings.Split(protoFqn, ".")
	msgName := protoFQN[len(protoFQN)-1]

	return fd.ParentFile().Messages().ByName(protoreflect.Name(msgName)), nil
}

// httpMemCache is a simple in-memory cache for http responses.
// It is used to avoid hitting the same URL multiple times.
// It configured to store data for 15 minutes and up until 100MB
var httpMemoryCache = lrucache.New(100<<(10*2), 60*15)

// getClient returns a http.Client that configured for retrying and caching with a maximum timeout of 5s.
func getClient() *http.Client {
	tr := httpcache.NewTransport(httpMemoryCache)
	tr.Transport = &retryablehttp.RoundTripper{}
	return &http.Client{
		Transport: httpcache.NewTransport(httpMemoryCache),
		Timeout:   5 * time.Second,
	}
}

// SchemaToFDs takes a Protobuf Schema or a URL of a Protobuf Schema and returns a list of FileDescriptors,
// the package name of the schama, the package name of the schema and an error if occurred.
func SchemaToFDs(schema string) ([]*desc.FileDescriptor, string, error) {
	filename := fmt.Sprintf("%s.proto", uuid.NewString())

	var rdr io.ReadCloser

	u, err := url.Parse(schema)
	if err == nil && u.Scheme != "" && u.Host != "" {
		if path.Ext(u.Path) != ".proto" {
			return nil, "", fmt.Errorf("doesn't support non-proto files")
		}
		filename = u.Host + u.Path

		resp, err := getClient().Get(schema)
		if err != nil {
			return nil, "", fmt.Errorf("unable to fetch schema from url(%s): %w", schema, err)
		}

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, "", fmt.Errorf("unable to fetch schema from url(%s), got status %d", schema, resp.StatusCode)
		}

		rdr = resp.Body
	} else {
		rdr = io.NopCloser(strings.NewReader(schema)) // r type is io.ReadCloser
	}

	parser := protoparse.Parser{Accessor: func(name string) (io.ReadCloser, error) {
		if name == filename {
			return rdr, nil
		}
		return os.Open(name)
	}}

	descriptors, err := parser.ParseFiles(filename)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse proto file: %w", err)
	}
	return descriptors, filename, err
}

// Register takes a Protobuf Schema or a URL of a Protobuf Schema and registers it in the global registry.
// It returns the package name of the schema and an error if occurred.
func Register(schema string) (string, error) {
	pack := ""
	descriptors, filename, err := SchemaToFDs(schema)
	if err != nil {
		return pack, fmt.Errorf("failed to parse proto schema: %w", err)
	}

	for _, pb := range descriptors {
		if pb.GetName() == filename {
			pack = pb.GetPackage()
		}
		fd, err := protodesc.NewFile(pb.AsFileDescriptorProto(), protoregistry.GlobalFiles)
		if err != nil {
			return pack, fmt.Errorf("create fd object: %w", err)
		}

		if _, err := protoregistry.GlobalFiles.FindFileByPath(filename); err != nil {
			err = protoregistry.GlobalFiles.RegisterFile(fd)
			if err != nil {
				return pack, fmt.Errorf("register fd: %w", err)
			}
		} else {
			return pack, ErrAlreadyRegistered
		}
	}

	return pack, nil
}
