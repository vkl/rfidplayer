package control

import (
	"fmt"
	"testing"
	"time"
)

func TestDiscovery(t *testing.T) {
	cc := NewChromeCastControl()
	cc.StartDiscovery(6 * time.Second)
	for i := 0; i < 2; i++ {
		fmt.Println(cc.GetClients())
		time.Sleep(2 * time.Second)
	}
}
