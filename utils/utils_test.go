package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/clowder/controllers/cloud.redhat.com/utils"
	core "k8s.io/api/core/v1"
)

func TestKafkaReconcilerFns(t *testing.T) {

}

func TestConverterFuncs(t *testing.T) {
	t.Run("Test intMin", func(t *testing.T) {
		answer, _ := utils.IntMin([]string{"4", "6", "7"})
		if answer != "4" {
			t.Errorf("Min function should have returned 4, returned %s", answer)
		}
	})
	t.Run("Test intMax", func(t *testing.T) {
		answer, _ := utils.IntMax([]string{"4", "6", "7"})
		if answer != "7" {
			t.Errorf("Min function should have returned 7, returned %s", answer)
		}
	})
	t.Run("Test ListMerge", func(t *testing.T) {
		answer, _ := utils.ListMerge([]string{"4,5,6", "6", "7,2"})
		if answer != "2,4,5,6,7" {
			t.Errorf("Min function should have returned 2,4,5,6,7 returned %s", answer)
		}
	})
}

func TestBase64Decode(t *testing.T) {
	s := core.Secret{
		Data: map[string][]byte{
			"key": []byte("bnVtYmVy"),
		},
	}
	decodedValue, _ := utils.B64Decode(&s, "key")
	assert.Equal(t, decodedValue, "number", "should decode the right value")
}

func TestRandString(t *testing.T) {
	a := utils.RandString(12)
	b := utils.RandString(12)
	assert.NotEqual(t, a, b)
}
