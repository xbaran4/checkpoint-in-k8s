package manager

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"context"
	"github.com/rs/zerolog/log"
	"time"
)

type defaultCheckpointManager struct {
	checkpointsInProgress *checkpointsInProgress
	checkpointer          checkpoint.Checkpointer
	checkpointStorage     CheckpointStorage
}

func (cm defaultCheckpointManager) Checkpoint(ctx context.Context, async bool, checkpointerParams checkpoint.CheckpointerParams) (*CheckpointEntry, error) {
	lg := log.With().Bool("async", async).Logger()

	if async {
		doneChan := make(chan struct{})
		cm.checkpointsInProgress.Put(checkpointerParams.CheckpointIdentifier, doneChan)
		go cm.doCheckpointAsync(checkpointerParams, doneChan)
		return nil, nil
	}

	beginTimestamp := time.Now().Unix()
	checkpointImageName, checkpointErr := cm.checkpointer.Checkpoint(lg.WithContext(ctx), checkpointerParams)

	if checkpointErr != nil {
		lg.Error().Err(checkpointErr).Msg("checkpointer failed")
		return nil, checkpointErr
	}

	return &CheckpointEntry{
		ContainerIdentifier: checkpointerParams.ContainerIdentifier,
		BeginTimestamp:      beginTimestamp,
		EndTimestamp:        time.Now().Unix(),
		ContainerImageName:  checkpointImageName,
	}, nil
}

func (cm defaultCheckpointManager) doCheckpointAsync(checkpointRequest checkpoint.CheckpointerParams, doneChan chan struct{}) {
	lg := log.With().Str("containerIdentifier", checkpointRequest.ContainerIdentifier.String()).Logger()

	beginTimestamp := time.Now().Unix()
	checkpointImageName, checkpointErr := cm.checkpointer.Checkpoint(context.Background(), checkpointRequest)
	if checkpointErr != nil {
		lg.Error().Err(checkpointErr).Msg("async checkpointer failed")
	}

	entry := CheckpointEntry{
		ContainerIdentifier: checkpointRequest.ContainerIdentifier,
		BeginTimestamp:      beginTimestamp,
		EndTimestamp:        time.Now().Unix(),
		ContainerImageName:  checkpointImageName,
		Error:               checkpointErr,
	}

	if err := cm.checkpointStorage.StoreEntry(checkpointRequest.CheckpointIdentifier, entry); err != nil {
		lg.Error().Err(err).Msg("failed to store checkpoint result, but in memory result is valid")
	}

	lg.Info().Msg("async checkpointer done, closing channel")
	cm.checkpointsInProgress.Delete(checkpointRequest.CheckpointIdentifier)
	close(doneChan)
}

func (cm defaultCheckpointManager) CheckpointResult(checkpointIdentifier string) (*CheckpointEntry, error) {
	lg := log.With().
		Str("checkpointIdentifier", checkpointIdentifier).
		Logger()

	doneChan := cm.checkpointsInProgress.Get(checkpointIdentifier)
	if doneChan != nil {
		_ = <-doneChan
	}

	entry, err := cm.checkpointStorage.ReadEntry(checkpointIdentifier)
	if err != nil {
		lg.Error().Err(err).Msg("failed to read checkpoint result")
		return nil, err
	}
	return entry, nil
}
