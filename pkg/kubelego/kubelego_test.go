package kubelego

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKubeLego_parseHostFilters(t *testing.T) {
	kl := New("test")

	kl.parseHostFilters()
	assert.Empty(t, kl.LegoHostFilters())

	os.Setenv("LEGO_HOST_FILTERS", "")
	kl.parseHostFilters()
	assert.Empty(t, kl.LegoHostFilters())

	os.Setenv("LEGO_HOST_FILTERS", ", ,\n")
	kl.parseHostFilters()
	assert.Empty(t, kl.LegoHostFilters())

	re1, err := regexp.Compile("[0-9]+")
	assert.Nil(t, err)
	re2, err := regexp.Compile("\\.+")
	assert.Nil(t, err)

	os.Setenv("LEGO_HOST_FILTERS", "  [0-9]+, \\.+,,\t\r")
	kl.parseHostFilters()
	assert.EqualValues(t, []*regexp.Regexp{re1, re2}, kl.LegoHostFilters())
}
