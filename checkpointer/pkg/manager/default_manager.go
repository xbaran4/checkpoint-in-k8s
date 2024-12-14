package manager

import (
	"checkpoint-in-k8s/pkg/checkpoint"
	"context"
	"github.com/rs/zerolog/log"
	"time"
)

type checkpointManager struct {

	// checkpointsInProgress is map of currently ongoing checkpoint goroutines.
	checkpointsInProgress *checkpointsInProgress

	// checkpointer is the checkpoint strategy this manager will use.
	checkpointer checkpoint.Checkpointer

	// checkpointStorage is where manager stores result of asynchronous checkpoints
	checkpointStorage CheckpointStorage
}

func (cm checkpointManager) Checkpoint(ctx context.Context, async bool, checkpointerParams checkpoint.CheckpointerParams) (*CheckpointEntry, error) {
	if !async {
		return cm.doCheckpoint(ctx, checkpointerParams)
	}

	doneChan := make(chan struct{})
	cm.checkpointsInProgress.Put(checkpointerParams.CheckpointIdentifier, doneChan)
	go cm.doCheckpointAsync(checkpointerParams, doneChan)
	return nil, nil
}

func (cm checkpointManager) doCheckpoint(ctx context.Context, checkpointerParams checkpoint.CheckpointerParams) (*CheckpointEntry, error) {
	lg := log.With().Bool("async", false).Logger()

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

func (cm checkpointManager) doCheckpointAsync(checkpointParams checkpoint.CheckpointerParams, doneChan chan struct{}) {
	lg := log.With().Str("containerIdentifier", checkpointParams.ContainerIdentifier.String()).Logger()

	beginTimestamp := time.Now().Unix()
	checkpointImageName, checkpointErr := cm.checkpointer.Checkpoint(lg.WithContext(context.Background()), checkpointParams)
	if checkpointErr != nil {
		lg.Error().Err(checkpointErr).Msg("async checkpointer failed")
	}

	entry := CheckpointEntry{
		ContainerIdentifier: checkpointParams.ContainerIdentifier,
		BeginTimestamp:      beginTimestamp,
		EndTimestamp:        time.Now().Unix(),
		ContainerImageName:  checkpointImageName,
		Error:               checkpointErr,
	}

	if err := cm.checkpointStorage.StoreEntry(checkpointParams.CheckpointIdentifier, entry); err != nil {
		lg.Error().Err(err).Msg("failed to store async checkpoint result, this is a PROBLEM")
	}

	lg.Info().Msg("async checkpoint done, closing channel")
	cm.checkpointsInProgress.Delete(checkpointParams.CheckpointIdentifier)
	close(doneChan)
}

func (cm checkpointManager) CheckpointResult(checkpointIdentifier string) (*CheckpointEntry, error) {
	lg := log.With().
		Str("checkpointIdentifier", checkpointIdentifier).
		Logger()

	if doneChan := cm.checkpointsInProgress.Get(checkpointIdentifier); doneChan != nil {
		_ = <-doneChan
	}

	entry, err := cm.checkpointStorage.ReadEntry(checkpointIdentifier)
	if err != nil {
		lg.Error().Err(err).Msg("failed to read checkpoint result")
		return nil, err
	}
	return entry, nil
}
