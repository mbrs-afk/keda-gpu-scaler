/*
Copyright 2026 The keda-gpu-scaler Authors.

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

package slurm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setEnv(t *testing.T, kvs map[string]string) {
	t.Helper()
	for k, v := range kvs {
		t.Setenv(k, v)
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "inside slurm job",
			env:  map[string]string{"SLURM_JOB_ID": "12345"},
			want: true,
		},
		{
			name: "outside slurm",
			env:  map[string]string{},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			setEnv(t, tt.env)
			assert.Equal(t, tt.want, Detect())
		})
	}
}

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want JobContext
	}{
		{
			name: "full slurm environment",
			env: map[string]string{
				"SLURM_JOB_ID":        "98765",
				"SLURM_JOB_NAME":      "train-llm",
				"SLURM_JOB_PARTITION": "gpu-a100",
				"SLURM_NODELIST":      "node[01-04]",
				"SLURM_NODENAME":      "node02",
				"SLURM_JOB_NUM_NODES": "4",
				"SLURM_NTASKS":        "32",
				"SLURM_PROCID":        "8",
				"SLURM_LOCALID":       "2",
				"SLURM_STEP_GPUS":     "0,1,2,3",
			},
			want: JobContext{
				JobID:     "98765",
				JobName:   "train-llm",
				Partition: "gpu-a100",
				NodeList:  "node[01-04]",
				NodeName:  "node02",
				NumNodes:  4,
				NumTasks:  32,
				ProcID:    8,
				LocalID:   2,
				GPUs:      "0,1,2,3",
			},
		},
		{
			name: "minimal - job id only",
			env: map[string]string{
				"SLURM_JOB_ID": "111",
			},
			want: JobContext{
				JobID: "111",
			},
		},
		{
			name: "empty env",
			env:  map[string]string{},
			want: JobContext{},
		},
		{
			name: "gpu fallback to CUDA_VISIBLE_DEVICES",
			env: map[string]string{
				"SLURM_JOB_ID":         "222",
				"CUDA_VISIBLE_DEVICES": "0,1",
			},
			want: JobContext{
				JobID: "222",
				GPUs:  "0,1",
			},
		},
		{
			name: "SLURM_STEP_GPUS takes priority over CUDA_VISIBLE_DEVICES",
			env: map[string]string{
				"SLURM_JOB_ID":         "333",
				"SLURM_STEP_GPUS":      "2,3",
				"CUDA_VISIBLE_DEVICES": "0,1,2,3",
			},
			want: JobContext{
				JobID: "333",
				GPUs:  "2,3",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			setEnv(t, tt.env)
			got := FromEnv()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVisibleDevices(t *testing.T) {
	tests := []struct {
		name string
		gpus string
		want []int
	}{
		{name: "multi gpu", gpus: "0,1,2,3", want: []int{0, 1, 2, 3}},
		{name: "single gpu", gpus: "2", want: []int{2}},
		{name: "empty", gpus: "", want: nil},
		{name: "with spaces", gpus: "0, 1, 3", want: []int{0, 1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := JobContext{GPUs: tt.gpus}
			assert.Equal(t, tt.want, j.VisibleDevices())
		})
	}
}

func TestHeaderRowAlignment(t *testing.T) {
	j := JobContext{
		JobID:   "100",
		JobName: "test",
		ProcID:  3,
	}
	assert.Equal(t, len(j.Header()), len(j.Row()))
}
