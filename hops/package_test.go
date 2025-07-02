// Copyright (c) 2023-2025, Nubificus LTD
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

package hops

import (
	//"context"
	"testing"

	//"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/client/llb"
	"github.com/stretchr/testify/require"
)

func TestPackLLB(t *testing.T) {
	t.Run("Empty instructions", func(t *testing.T) {
		instr := PackInstructions{
			Base: llb.Scratch(),
		}
		def, err := PackLLB(instr)
		require.NoError(t, err)
		require.NotNil(t, def)
	})
}
