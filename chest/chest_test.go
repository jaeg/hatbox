package chest

import (
	"testing"

	"github.com/alicebob/miniredis"
	_ "github.com/robertkrimen/otto/underscore"
)

func TestStartErrorWithNoRedisAddress(t *testing.T) {
	_, err := Create("", "", "", "TestCluster", "TestChest", "9999", "8787")
	if err.Error() != "no redis address provided" {
		t.Errorf("Did not fail due to no redis address.")
	}
}

func TestStartErrorWithFailedPing(t *testing.T) {
	_, err := Create("", "bad", "", "TestCluster", "TestChest", "9999", "8787")
	if err.Error() != "redis failed ping" {
		t.Errorf("Did not fail due to failed ping.")
	}
}

func TestStartReturnsNilWhenSuccessful(t *testing.T) {
	mr, _ := miniredis.Run()
	_, err := Create("", mr.Addr(), "", "TestCluster", "TestChest", "9999", "8787")
	if err != nil {
		t.Errorf("Errored starting chest.")
	}
}
