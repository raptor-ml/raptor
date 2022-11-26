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

package redis

import (
	"context"
	"github.com/go-redis/redis/v8"
)

type redisScripts []*redis.Script

func (rs redisScripts) Load(scripter redis.Scripter) error {
	for _, s := range rs {
		if err := s.Load(context.Background(), scripter).Err(); err != nil {
			return err
		}
	}
	return nil
}

var scripts = redisScripts{luaHMax, luaHMin, luaMax, luaMaxExpAt}

// luaHMin doing an atomic MIN operation on a given Hash's Field
// Arguments:
//   - KEYS[1] - Hash Key
//   - KEYS[2] - Field key
//   - ARGV[1] - Numeric Value
//
// Returns 1 if there was a change or 0 if not
var luaHMin = redis.NewScript(`
local key = KEYS[1]
local field = KEYS[2]
local num = tonumber(ARGV[1])

local value = redis.call('HGET', key, field)
if not value or num < tonumber(value) then
  return redis.call('HSET', key, field, num)
end

return 0
`)

// luaHMax doing an atomic MAX operation on a given Hash's Field
// Arguments:
//   - KEYS[1] - Hash Key
//   - KEYS[2] - Field key
//   - ARGV[1] - Numeric Value
//
// Returns 1 if there was a change or 0 if not
var luaHMax = redis.NewScript(`
local key = KEYS[1]
local field = KEYS[2]
local num = tonumber(ARGV[1])

local value = redis.call('HGET', key, field)
if not value or num > tonumber(value) then
  return redis.call('HSET', key, field, num)
end

return 0
`)

// luaMax doing an atomic MAX operation on a regular key
// Arguments:
//   - KEYS[1] - Key
//   - ARGV[1] - Numeric Value
//   - ARGV[2] - Optional expiration
//
// Returns 1 if there was a change or 0 if not
var luaMax = redis.NewScript(`
local key = KEYS[1]
local num = tonumber(ARGV[1])
local ttl = ARGV[2]

local value = redis.call('GET', key)
if not value or num > tonumber(value) then
  if not not ttl then
    local dur = tonumber(ttl)
    if dur > 0 or dur == -1 then
      return redis.call('SET', key, num, 'PX', ttl)
    end
  end
  return redis.call('SET', key, num)
end

return 0
`)

// luaMaxExpAt doing an atomic MAX operation on a regular key
// Arguments:
//   - KEYS[1] - Key
//   - ARGV[1] - Numeric Value
//   - ARGV[2] - ExpireAt
//
// Returns 1 if there was a change or 0 if not
var luaMaxExpAt = redis.NewScript(`
local key = KEYS[1]
local num = tonumber(ARGV[1])
local xat = ARGV[2]

local value = redis.call('GET', key)
if not value or num > tonumber(value) then
  redis.call('SET', key, num, 'PXAT', xat)
end

return 0
`)
