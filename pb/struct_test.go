/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package pb_test

import (
	"testing"

	types "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"

	"github.com/sniperkit/snk.fork.lookout/pb"
)

func TestToStruct(t *testing.T) {
	require := require.New(t)

	inputMap := map[string]interface{}{
		"bool":   true,
		"int":    1,
		"string": "val",
		"float":  0.5,
		"nil":    nil,
		"array":  []string{"val1", "val2"},
		"map": map[string]int{
			"field1": 1,
		},
		"struct": struct {
			Val string
		}{Val: "val"},
	}

	expectedSt := &types.Struct{
		Fields: map[string]*types.Value{
			"bool": &types.Value{
				Kind: &types.Value_BoolValue{
					BoolValue: true,
				},
			},
			"int": &types.Value{
				Kind: &types.Value_NumberValue{
					NumberValue: 1,
				},
			},
			"string": &types.Value{
				Kind: &types.Value_StringValue{
					StringValue: "val",
				},
			},
			"float": &types.Value{
				Kind: &types.Value_NumberValue{
					NumberValue: 0.5,
				},
			},
			"nil": nil,
			"array": &types.Value{
				Kind: &types.Value_ListValue{
					ListValue: &types.ListValue{
						Values: []*types.Value{
							&types.Value{
								Kind: &types.Value_StringValue{
									StringValue: "val1",
								},
							},
							&types.Value{
								Kind: &types.Value_StringValue{
									StringValue: "val2",
								},
							},
						},
					},
				},
			},
			"map": &types.Value{
				Kind: &types.Value_StructValue{
					StructValue: &types.Struct{
						Fields: map[string]*types.Value{
							"field1": &types.Value{
								Kind: &types.Value_NumberValue{
									NumberValue: 1,
								},
							},
						},
					},
				},
			},
			"struct": &types.Value{
				Kind: &types.Value_StructValue{
					StructValue: &types.Struct{
						Fields: map[string]*types.Value{
							"Val": &types.Value{
								Kind: &types.Value_StringValue{
									StringValue: "val",
								},
							},
						},
					},
				},
			},
		},
	}

	st := pb.ToStruct(inputMap)
	require.Equal(expectedSt, st)
}
