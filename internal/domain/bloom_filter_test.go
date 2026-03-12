package domain_test

import (
	"fmt"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestBloomFilter_Add_And_MayContain(t *testing.T) {
	// given
	bf := domain.NewBloomFilter(1000, 0.01)

	// when
	bf.Add("hello")

	// then
	if !bf.MayContain("hello") {
		t.Error("MayContain should return true for added element")
	}
}

func TestBloomFilter_MayContain_ReturnsFalseForAbsent(t *testing.T) {
	// given
	bf := domain.NewBloomFilter(1000, 0.01)

	// then
	if bf.MayContain("never-added") {
		t.Error("MayContain should return false for element never added")
	}
}

func TestBloomFilter_NoFalseNegatives(t *testing.T) {
	// given
	bf := domain.NewBloomFilter(10000, 0.01)
	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%04d", i)
		bf.Add(keys[i])
	}

	// then: every added key must be found (zero false negatives)
	for _, k := range keys {
		if !bf.MayContain(k) {
			t.Errorf("false negative for %q", k)
		}
	}
}

func TestBloomFilter_MarshalUnmarshal_PreservesState(t *testing.T) {
	// given
	bf := domain.NewBloomFilter(1000, 0.01)
	bf.Add("alpha")
	bf.Add("beta")

	// when
	data := bf.MarshalBinary()
	restored, err := domain.UnmarshalBloomFilter(data)

	// then
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !restored.MayContain("alpha") {
		t.Error("restored filter lost 'alpha'")
	}
	if !restored.MayContain("beta") {
		t.Error("restored filter lost 'beta'")
	}
	if restored.MayContain("gamma") {
		t.Error("restored filter has false positive for 'gamma' — possible but check hash")
	}
}

func TestBloomFilter_UnmarshalCorrupt_ReturnsError(t *testing.T) {
	// given
	corrupt := []byte{0x00, 0x01, 0x02}

	// when
	_, err := domain.UnmarshalBloomFilter(corrupt)

	// then
	if err == nil {
		t.Error("expected error for corrupt data")
	}
}

func TestBloomFilter_FalsePositiveRate_WithinBounds(t *testing.T) {
	// given: BF sized for 10000 elements at 1% FPR
	bf := domain.NewBloomFilter(10000, 0.01)
	for i := 0; i < 10000; i++ {
		bf.Add(fmt.Sprintf("added-%d", i))
	}

	// when: test 100000 elements that were NOT added
	falsePositives := 0
	testN := 100000
	for i := 0; i < testN; i++ {
		if bf.MayContain(fmt.Sprintf("absent-%d", i)) {
			falsePositives++
		}
	}

	// then: FPR should be < 2% (generous margin over theoretical 1%)
	rate := float64(falsePositives) / float64(testN)
	if rate > 0.02 {
		t.Errorf("false positive rate %.4f exceeds 2%% threshold", rate)
	}
}
