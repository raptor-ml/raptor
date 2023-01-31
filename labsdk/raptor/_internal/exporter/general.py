# -*- coding: utf-8 -*-
#  Copyright (c) 2022 RaptorML authors.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

import os
from datetime import datetime
from typing import Dict

from jinja2 import Template, FileSystemLoader, Environment

from ... import local_state
from ...types.model import ModelSpec


class _GeneralExporter:
    models: Dict[str, ModelSpec] = {}
    features: Dict[str, 'FeatureSpec'] = {}
    sources: Dict[str, 'DataSourceSpec'] = {}

    """
    Environment variables that needs to be set to configure the exported files.
    This contains a dictionary of environment variable name, and a description of the variable.
    """
    envs: Dict[str, str] = {}
    _makefile: Template = None

    @property
    def makefile(self) -> Template:
        if self._makefile is None:
            env = Environment(loader=FileSystemLoader(os.path.dirname(__file__)))
            env.filters['targetize'] = lambda s: s.replace(':', '-').replace('/', '-').replace('.', '-').lower()
            env.globals['now'] = datetime.utcnow
            template = env.get_template('Makefile.j2')
            self._makefile = template
        return self._makefile

    def add(self, spec: 'RaptorSpec'):
        typ = spec.__class__.__name__
        if typ == 'ModelSpec' or typ == 'ModelImpl' or isinstance(spec, ModelSpec):
            self.add_model(spec)
        elif typ == 'FeatureSpec':
            self.add_feature(spec)
        elif typ == 'DataSourceSpec':
            self.add_source(spec)
        else:
            raise TypeError(f'Cannot add {spec} to exporter. {typ} is not supported by the exporter.')

    def add_model(self, model: ModelSpec, with_dependent_features=True, with_dependent_sources=True):
        self.models[model.fqn()] = model

        if with_dependent_features:
            for feature in model.features:
                self.add_feature(local_state.feature_spec_by_selector(selector=feature),
                                 with_dependent_source=with_dependent_sources)

    def add_feature(self, feature: 'FeatureSpec', with_dependent_source=True):
        self.features[feature.fqn()] = feature
        if with_dependent_source and feature.data_source_spec is not None:
            self.add_source(feature.data_source_spec)

    def add_source(self, source: 'DataSourceSpec'):
        self.sources[source.fqn()] = source

    def add_env(self, name: str, description: str):
        if name.startswith('$'):
            name = name[1:]
        self.envs[name] = description

    def export(self, with_makefile=True, remove_unused_models=True):
        for _, model in self.models.items():
            model.exporter.export(with_docker=True, remove_unused_models=remove_unused_models)

        for _, feature in self.features.items():
            feature.manifest(to_file=True)

        for _, source in self.sources.items():
            source.manifest(to_file=True)

        if with_makefile:
            output = self.makefile.render(models=self.models, envs=self.envs, features=self.features,
                                          sources=self.sources)
            with open(os.path.join(os.getcwd(), 'out', 'Makefile'), 'w') as f:
                f.write(output)
                f.flush()


GeneralExporter = _GeneralExporter()
