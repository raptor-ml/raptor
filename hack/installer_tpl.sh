#!/bin/bash

# Copyright (c) 2022 RaptorML authors.
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

CONFIG_CORE_ARGS=""
function config_core_args() {
  read -p "Enter Core configuration arguments: " CONFIG_CORE_ARGS
  if [ -z "$CONFIG_CORE_ARGS" ]; then
    echo ""
    echo -e "\033[0;31mYou must specify Core configuration arguments\033[0m"
    config_core_args
  fi
}
config_core_args
echo "Core Configuration arguments: $CONFIG_CORE_ARGS"
echo ""

HISTORIAN_REPLICAS=1
CONFIG_HISTORIAN_ARGS=""
function config_historian_args() {
  read -p "Enter Historian configuration arguments (write --disable to disable the historian): " CONFIG_HISTORIAN_ARGS
  if [ -z "$CONFIG_HISTORIAN_ARGS" ]; then
    echo ""
    echo -e "\033[0;31mYou must specify Historian configuration arguments\033[0m"
    config_historian_args
  fi
  if [ "$CONFIG_HISTORIAN_ARGS" == "--disable" ]; then
    echo ""
    echo -e "\033[1;33mHistorian is disabled\033[0m"
    CONFIG_HISTORIAN_ARGS="--production=true"
    HISTORIAN_REPLICAS=0
  fi
}
config_historian_args
echo "Historian Configuration arguments: $CONFIG_HISTORIAN_ARGS"
echo ""

read -p "Are you sure you want to install using context \`$(kubectl config current-context)\`? [y/N] " -n 1 -r
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborting."
  exit 1
fi
echo ""
echo ""
echo -e "\033[1;34mInstalling manifests...\033[0m"

function manifests() {
    echo ${MANIFESTS} | base64 -d | sed "s/\$installer_core_args\\$/${CONFIG_CORE_ARGS}/g" | sed "s/replicas: -123/replicas: ${HISTORIAN_REPLICAS}/g" | sed "s/\$installer_historian_args\\$/${CONFIG_HISTORIAN_ARGS}/g"
}
manifests | kubectl apply -f -

exit 0
__MANIFESTS__
