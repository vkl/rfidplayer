package control

import (
	"fmt"
	"net"
	"testing"
)

func TestCasts(t *testing.T) {
	c, err := NewCastController("debug.json")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(c.GetCasts())
	if err := c.AddCast(Cast{
		Name:   "test2",
		IPAddr: net.IPv4(192, 168, 1, 111),
		Port:   8009,
	}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(c.GetCasts())
}
