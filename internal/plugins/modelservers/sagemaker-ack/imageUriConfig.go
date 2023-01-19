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

package sagemaker_ack

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

//go:embed image_uri_config
var configs embed.FS
var frameworkConfigs = make(map[string]FrameworkConfig)

func init() {
	de, err := configs.ReadDir("image_uri_config")
	if err != nil {
		panic(fmt.Errorf("failed to read image_uri_config directory: %w", err))
	}

	for _, d := range de {
		if d.IsDir() {
			continue
		}
		f, err := configs.Open("image_uri_config/" + d.Name())
		if err != nil {
			panic(fmt.Errorf("failed to open image_uri_config/%s: %w", d.Name(), err))
		}

		fc := FrameworkConfig{}
		err = json.NewDecoder(f).Decode(&fc)
		if err != nil {
			panic(fmt.Errorf("failed to decode image_uri_config/%s: %w", d.Name(), err))
		}

		fw := d.Name()[0 : len(d.Name())-len(path.Ext(d.Name()))]
		frameworkConfigs[fw] = fc
	}
}

func ImageURI(framework, region, frameworkVer string) (string, error) {
	fc, ok := frameworkConfigs[framework]
	if !ok {
		return "", fmt.Errorf("no framework config found for %s", framework)
	}

	ret, err := fc.Image(region, frameworkVer)
	if err != nil {
		return "", fmt.Errorf("failed to get image for framework %s version %s in region %s: %w", framework, frameworkVer, region, err)
	}
	return ret, nil
}

type FrameworkConfig struct {
	Inference         *Scope `json:"inference,omitempty"`
	Training          *Scope `json:"training,omitempty"`
	InferenceGraviton *Scope `json:"inference_graviton,omitempty"`
}
type Version struct {
	Processors       []string          `json:"processors,omitempty"`
	PyVersions       []string          `json:"py_versions,omitempty"`
	Registries       map[string]string `json:"registries,omitempty"`
	ContainerVersion map[string]string `json:"container_version,omitempty"`
	Repository       string            `json:"repository,omitempty"`
	TagPrefix        string            `json:"tag_prefix,omitempty"`

	VersionAliases map[string]string  `json:"version_aliases,omitempty"`
	Versions       map[string]Version `json:"-"`
}

func (m *Version) UnmarshalJSON(data []byte) error {
	type Alias Version
	if err := json.Unmarshal(data, (*Alias)(m)); err != nil {
		return err
	}

	aux := map[string]Alias{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return nil
	}
	m.Versions = map[string]Version{}
	for k, v := range aux {
		switch k {
		case "processors", "py_versions", "registries", "container_version", "repository", "tag_prefix", "version_aliases":
			continue
		default:
			m.Versions[k] = Version(v)
		}
	}
	return nil
}

type Scope struct {
	Processors     []string           `json:"processors,omitempty"`
	VersionAliases map[string]string  `json:"version_aliases,omitempty"`
	Versions       map[string]Version `json:"versions,omitempty"`
}

func (s Scope) Image(region string, ver string) (string, error) {
	verParts := strings.SplitN(ver, "+", 2)
	ver = verParts[0]
	baseVer := ""
	if len(verParts) > 1 {
		baseVer = verParts[1]
	}

	if s.Processors != nil {
		cpu := false
		for _, p := range s.Processors {
			if p == "cpu" {
				cpu = true
				break
			}
		}
		if !cpu {
			return "", fmt.Errorf("cpu processor is not supported")
		}
	}
	if s.VersionAliases != nil {
		for alias, av := range s.VersionAliases {
			if alias == ver {
				ver = av
				break
			}
		}
	}
	if s.Versions == nil {
		return "", fmt.Errorf("no versions found")
	}
	v, ok := s.Versions[ver]
	if !ok {
		return "", fmt.Errorf("version %s is not supported", ver)
	}
	if baseVer != "" {
		if v.Versions == nil {
			return "", fmt.Errorf("no versions found")
		}

		if v.VersionAliases != nil {
			for alias, av := range v.VersionAliases {
				if alias == baseVer {
					baseVer = av
					break
				}
			}
		}
		v, ok = v.Versions[baseVer]
		if !ok {
			return "", fmt.Errorf("version %s is not supported", baseVer)
		}
	}

	if v.Processors != nil {
		cpu := false
		for _, p := range v.Processors {
			if p == "cpu" {
				cpu = true
				break
			}
		}
		if !cpu {
			return "", fmt.Errorf("cpu processor is not supported for version %s", ver)
		}
	}
	pyver := "py3"
	if v.PyVersions != nil {
		for _, p := range v.PyVersions {
			if strings.HasPrefix(p, "py3") {
				pyver = p
			}
		}
	}
	if v.Registries == nil {
		return "", fmt.Errorf("no registries found for version %s", ver)
	}
	registry, ok := v.Registries[region]
	if !ok {
		return "", fmt.Errorf("no registry found for version %s in region %s", ver, region)
	}

	if v.Repository == "" {
		return "", fmt.Errorf("no repository found for version %s in region %s", ver, region)
	}

	tag := fmt.Sprintf("%s-cpu-%s", ver, pyver)
	if v.ContainerVersion != nil {
		cv, ok := v.ContainerVersion["cpu"]
		if !ok {
			return "", fmt.Errorf("no container version found for cpu processor for version %s in region %s", ver, region)
		}
		tag = fmt.Sprintf("%s-%s", tag, cv)
	}

	if strings.HasPrefix(v.Repository, "huggingface-") {
		if baseVer == "" {
			return "", fmt.Errorf("no base version found for version %s in region %s", ver, region)
		}
		fwver := strings.TrimPrefix(strings.TrimPrefix(baseVer, "pytorch"), "tensorflow")
		tag = fmt.Sprintf("%s-transformers%s-cpu-%s", fwver, ver, pyver)
	}

	if v.TagPrefix != "" {
		tag = fmt.Sprintf("%s-%s", v.TagPrefix, tag)
	}

	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s", registry, region, v.Repository, tag), nil
}

func (cfg *FrameworkConfig) Image(region, frameworkVer string) (string, error) {
	if cfg.Inference == nil {
		return "", fmt.Errorf("inference scope is not defined")
	}
	return cfg.Inference.Image(region, frameworkVer)
}
