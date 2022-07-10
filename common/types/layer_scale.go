// Code generated by github.com/spacemeshos/go-scale/scalegen. DO NOT EDIT.

package types

import (
	"github.com/spacemeshos/go-scale"
)

func (t *LayerID) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := scale.EncodeCompact32(enc, t.Value); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *LayerID) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if field, n, err := scale.DecodeCompact32(dec); err != nil {
		return total, err
	} else {
		total += n
		t.Value = field
	}
	return total, nil
}