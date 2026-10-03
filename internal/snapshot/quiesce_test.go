package snapshot

import (
	"errors"
	"reflect"
	"testing"
)

func stubDocker(t *testing.T, using map[string][]string, failOn string) *[]string {
	t.Helper()
	var calls []string
	prevUsing, prevAction := runningContainersUsing, containerAction
	runningContainersUsing = func(volume string) ([]string, error) { return using[volume], nil }
	containerAction = func(action string, containers []string) error {
		for _, c := range containers {
			calls = append(calls, action+" "+c)
		}
		if action == failOn {
			return errors.New("boom")
		}
		return nil
	}
	t.Cleanup(func() { runningContainersUsing, containerAction = prevUsing, prevAction })
	return &calls
}

func TestQuiesce(t *testing.T) {
	vols := []VolumeSnapshot{{VolumeName: "pg"}, {VolumeName: "shared"}, {VolumeName: "idle"}}

	t.Run("stops each user once and starts it again", func(t *testing.T) {
		calls := stubDocker(t, map[string][]string{"pg": {"db"}, "shared": {"db", "cache"}}, "")

		resume, err := quiesce(vols)
		if err != nil {
			t.Fatal(err)
		}
		resume()

		want := []string{"stop cache", "stop db", "start cache", "start db"}
		if !reflect.DeepEqual(*calls, want) {
			t.Errorf("calls = %v, want %v", *calls, want)
		}
	})

	t.Run("nothing running, nothing touched", func(t *testing.T) {
		calls := stubDocker(t, nil, "")
		resume, err := quiesce(vols)
		if err != nil {
			t.Fatal(err)
		}
		resume()
		if len(*calls) != 0 {
			t.Errorf("no container uses the volumes, got %v", *calls)
		}
	})

	t.Run("a failed stop aborts the restore", func(t *testing.T) {
		stubDocker(t, map[string][]string{"pg": {"db"}}, "stop")
		if _, err := quiesce(vols); err == nil {
			t.Error("restoring under a container that would not stop must fail")
		}
	})
}
