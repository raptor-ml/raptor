#!/bin/bash

# Copyright 2022 Natun.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

MANIFESTS=$(sed '1,/^__MANIFESTS__/d' $0)

read -p "Enter configuration arguments: " CONFIG_ARGS
if [ -z "$CONFIG_ARGS" ]; then
  echo "You must specify configuration arguments"
  exit 1
fi
echo "Configuration arguments: $CONFIG_ARGS"

read -p "Are you sure you want to install using context \`$(kubectl config current-context)\`? [y/N] " -n 1 -r
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborting."
  exit 1
fi
echo ""
echo "Installing manifests..."

echo ${MANIFESTS} | base64 -d | sed "s/\$installer_args\\$/${CONFIG_ARGS}/g" | kubectl apply -f -

exit 0
__MANIFESTS__
