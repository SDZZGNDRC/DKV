package trie_test

import (
	"log"
	"testing"

	"github.com/SDZZGNDRC/DKV/src/pkg/trie"
)

func TestTeri(test *testing.T) {
	t := trie.NewTrieX()
	t.Put("a", "1")
	t.Put("ab", "2")
	t.Put("abb", "3")
	t.Put("b", "4")
	n, ok := t.Get("a")
	log.Println("v = ", n, "ok =", ok)
	n, ok = t.Get("c")
	log.Println("v = ", n, "ok =", ok)
	data, err := t.Marshal()
	if err != nil {
		test.Error(err)
	}
	err = t.UmMarshal(data)
	if err != nil {
		test.Error(err)
	}
	rt := t.GetWithPrefix("a")
	log.Println("v = ", rt)

}
