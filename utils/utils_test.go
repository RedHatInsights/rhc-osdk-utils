package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	core "k8s.io/api/core/v1"
)

func TestConverterFuncs(t *testing.T) {
	t.Run("Test intMin", func(t *testing.T) {
		answer, _ := IntMin([]string{"4", "6", "7"})
		if answer != "4" {
			t.Errorf("Min function should have returned 4, returned %s", answer)
		}
	})
	t.Run("Test intMax", func(t *testing.T) {
		answer, _ := IntMax([]string{"4", "6", "7"})
		if answer != "7" {
			t.Errorf("Min function should have returned 7, returned %s", answer)
		}
	})
	t.Run("Test ListMerge", func(t *testing.T) {
		answer, _ := ListMerge([]string{"4,5,6", "6", "7,2"})
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
	decodedValue, _ := B64Decode(&s, "key")
	assert.Equal(t, decodedValue, "number", "should decode the right value")
}

func TestRandString(t *testing.T) {
	a := RandString(12)
	b := RandString(12)
	assert.NotEqual(t, a, b)
}

func TestRandStringLower(t *testing.T) {
	a := RandStringLower(12)
	b := RandStringLower(12)
	assert.NotEqual(t, a, b)
}

func TestRandPass(t *testing.T) {
	a, err1 := RandPassword(16)
	b, err2 := RandPassword(16)
	assert.NotEqual(t, a, b)
	assert.NoError(t, err1)
	assert.NoError(t, err2)

	c, err := RandPassword(12)
	assert.Error(t, err)
	assert.Equal(t, "", c)
}

func TestRandPassMinimal(t *testing.T) {
	a, _ := RandPassword(16, "abcd")
	for _, ch := range a {
		assert.Contains(t, "abcd", string(ch))
	}
}

type TestAnnotator struct {
	annotations map[string]string
}

func (a *TestAnnotator) SetAnnotations(annos map[string]string) {
	a.annotations = annos
}

func (a *TestAnnotator) GetAnnotations() map[string]string {
	return a.annotations
}

func TestAnnotatorSingle(t *testing.T) {
	initAnnos := map[string]string{
		"test": "colour me green",
	}
	b := &TestAnnotator{annotations: initAnnos}
	UpdateAnnotations(b, map[string]string{
		"test2": "ready steady restart",
	})

	expected := map[string]string{
		"test":  "colour me green",
		"test2": "ready steady restart",
	}

	assert.Equal(t, expected, b.GetAnnotations())
}

func TestAnnotatorMulti(t *testing.T) {
	initAnnos := map[string]string{
		"test": "colour me green",
	}

	b := &TestAnnotator{annotations: initAnnos}
	UpdateAnnotations(b,
		map[string]string{
			"test2": "ready steady restart",
		},
		map[string]string{
			"test3": "with a 1,2,3",
		})

	expected := map[string]string{
		"test":  "colour me green",
		"test2": "ready steady restart",
		"test3": "with a 1,2,3",
	}

	assert.Equal(t, expected, b.GetAnnotations())
}
