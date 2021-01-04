package committeestate

import (
	"reflect"
	"testing"

	"github.com/incognitochain/incognito-chain/instruction"
)

func TestBeaconCommitteeEngineV3_GenerateAssignInstruction(t *testing.T) {
	type fields struct {
		beaconCommitteeEngineSlashingBase beaconCommitteeEngineSlashingBase
	}
	type args struct {
		rand         int64
		assignOffset int
		activeShards int
		beaconHeight uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*instruction.AssignInstruction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &BeaconCommitteeEngineV3{
				beaconCommitteeEngineSlashingBase: tt.fields.beaconCommitteeEngineSlashingBase,
			}
			if got := engine.GenerateAssignInstruction(tt.args.rand, tt.args.assignOffset, tt.args.activeShards, tt.args.beaconHeight); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BeaconCommitteeEngineV3.GenerateAssignInstruction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBeaconCommitteeEngineV3_UpdateCommitteeState(t *testing.T) {
	type fields struct {
		beaconCommitteeEngineSlashingBase beaconCommitteeEngineSlashingBase
	}
	type args struct {
		env *BeaconCommitteeStateEnvironment
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *BeaconCommitteeStateHash
		want1   *CommitteeChange
		want2   [][]string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &BeaconCommitteeEngineV3{
				beaconCommitteeEngineSlashingBase: tt.fields.beaconCommitteeEngineSlashingBase,
			}
			got, got1, got2, err := engine.UpdateCommitteeState(tt.args.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("BeaconCommitteeEngineV3.UpdateCommitteeState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BeaconCommitteeEngineV3.UpdateCommitteeState() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("BeaconCommitteeEngineV3.UpdateCommitteeState() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("BeaconCommitteeEngineV3.UpdateCommitteeState() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestBeaconCommitteeEngineV3_UpdateCommitteeState_MultipleInstructions(t *testing.T) {
	type fields struct {
		beaconCommitteeEngineSlashingBase beaconCommitteeEngineSlashingBase
	}
	type args struct {
		env *BeaconCommitteeStateEnvironment
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *BeaconCommitteeStateHash
		want1   *CommitteeChange
		want2   [][]string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &BeaconCommitteeEngineV3{
				beaconCommitteeEngineSlashingBase: tt.fields.beaconCommitteeEngineSlashingBase,
			}
			got, got1, got2, err := engine.UpdateCommitteeState(tt.args.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("BeaconCommitteeEngineV3.UpdateCommitteeState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BeaconCommitteeEngineV3.UpdateCommitteeState() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("BeaconCommitteeEngineV3.UpdateCommitteeState() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("BeaconCommitteeEngineV3.UpdateCommitteeState() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
