//
// Copyright 2021 The Sigstore Authors.
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

package git

import (
	"context"

	"github.com/franchb/cosign/v2/pkg/cosign"
	"github.com/franchb/cosign/v2/pkg/cosign/git/github"
	"github.com/franchb/cosign/v2/pkg/cosign/git/gitlab"
)

var providerMap = map[string]Git{
	github.ReferenceScheme: github.New(),
	gitlab.ReferenceScheme: gitlab.New(),
}

type Git interface {
	PutSecret(ctx context.Context, ref string, pf cosign.PassFunc) error
	GetSecret(ctx context.Context, ref string, key string) (string, error)
}

func GetProvider(provider string) Git {
	return providerMap[provider]
}
