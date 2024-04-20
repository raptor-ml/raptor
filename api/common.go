/*
 * Copyright (c) 2022 RaptorML authors.
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

package api

import (
	"fmt"
	"regexp"
	"strconv"
)

var FQNRegExp = regexp.MustCompile(`(?si)^((?P<namespace>[a-z0-9]+(?:_[a-z0-9]+)*)\.)?(?P<name>[a-z0-9]+(?:_[a-z0-9]+)*)(\+(?P<aggrFn>([a-z]+_*[a-z]+)))?(@-(?P<version>([0-9]+)))?(\[(?P<encoding>([a-z]+_*[a-z]+))])?$`)

func ParseSelector(fqn string) (namespace, name string, aggrFn AggrFn, version uint, encoding string, err error) {
	if !FQNRegExp.MatchString(fqn) {
		return "", "", AggrFnUnknown, 0, "", fmt.Errorf("invalid FQN: %s", fqn)
	}

	match := FQNRegExp.FindStringSubmatch(fqn)
	parsedFQN := make(map[string]string)
	for i, name := range FQNRegExp.SubexpNames() {
		if i != 0 && name != "" {
			parsedFQN[name] = match[i]
		}
	}

	var ver = 0
	if parsedFQN["version"] != "" {
		ver, err = strconv.Atoi(parsedFQN["version"])
		if err != nil {
			return "", "", AggrFnUnknown, 0, "", fmt.Errorf("invalid version: %s", parsedFQN["version"])
		}
		if ver < 0 {
			ver *= -1
		}
	}

	namespace = parsedFQN["namespace"]
	name = parsedFQN["name"]
	aggrFn = StringToAggrFn(parsedFQN["aggrFn"])
	version = uint(ver)
	encoding = parsedFQN["encoding"]
	return
}

// NormalizeFQN returns an FQN with the namespace
func NormalizeFQN(fqn, defaultNamespace string) (string, error) {
	namespace, name, _, _, _, err := ParseSelector(fqn)
	if err != nil {
		return "", err
	}
	if namespace == "" {
		namespace = defaultNamespace
	}
	return fmt.Sprintf("%s.%s", namespace, name), nil
}

// NormalizeSelector returns a selector with the default namespace if not specified
func NormalizeSelector(selector, defaultNamespace string) (string, error) {
	ns, name, aggrFn, version, enc, err := ParseSelector(selector)
	if err != nil {
		return "", err
	}

	if ns == "" {
		ns = defaultNamespace
	}

	other := ""
	if aggrFn != AggrFnUnknown {
		other = fmt.Sprintf("%s+%s", other, aggrFn)
	}
	if version != 0 {
		other = fmt.Sprintf("%s@-%d", other, version)
	}
	if enc != "" {
		other = fmt.Sprintf("%s[%s]", other, enc)
	}
	return fmt.Sprintf("%s.%s%s", ns, name, other), nil
}
