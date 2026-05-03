// Copyright 2026 The SRE Portal Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slackclient_test

// Shared string literals used across tests in this package, extracted to
// satisfy the goconst linter (min-occurrences: 3).
const (
	tNameEmoji         = "emoji"
	tURLEmojiRabbitMQ  = "https://emoji.slack.com/rabbitmq.png"
	tURLEmojiOriginal  = "https://emoji.slack.com/original.png"
	tURLEmojiReal      = "https://emoji.slack.com/real.png"
	tEmojiNameRabbitMQ = "rabbitmq"
	tEmojiNameReal     = "real"
)
