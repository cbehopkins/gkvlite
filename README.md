gkvlite
-------

gkvlite is a simple, ordered, key-value persistence library for Go

Overview
========

gkvlite is a library that provides a simple key-value persistence
implementation, inspired by SQLite and CouchDB/Couchstore.

gkvlite has the following features...

* 100% implemented in the Go Language (golang).
* Open source license - MIT.
* Keys are ordered, so range iterations are supported.
* Keys are []byte.
* Values are []byte.
* On-disk storage is a single file.
* O(log N) performance for item retrieval.
* O(log N) performance to find the smallest or largest items (by key).
* Multiple key-value collections are supported in a single storage file.
* Append-only, copy-on-write design for robustness to crashes/power-loss.
* Read-only snapshots are supported so concurrent readers won't block writers.
* In-memory-only mode, when you want the same API but without any persistence.
* Single-threaded - users might use Go channels to serialize their own accesses.
* You provide the os.File.
* You provide the os.File.Sync(), if you want it.
* You control when you Flush() to disk, for performance-vs-safety tradeoffs.
* You can retrieve just keys only, to save I/O & memory resources when values are large.
* You can supply your own KeyCompare function to order items however you want.
* You can control item priority to access hotter items faster
  by shuffling them closer to the tops of balanced binary
  trees (warning: intricate/advanced tradeoffs here).
* Small - the implementation is a single file < 1000 lines of code.

Tips
====

To get a probabilistic O(log N) balanced tree height, you should use a
random priority number (e.g., rand.Int()) during the Upsert()
operation.  See Examples.

LICENSE
=======

MIT

Examples
========

    import (
        "math/rand"
        "os"
        "github.com/steveyen/gkvlite"
    )
    
	f, err := os.Create("/tmp/test.gkvlite")
	s, err := NewStore(f)
	c := s.SetCollection("cars", nil)
    
    c.Upsert(&gkvlite.Item{
        Key: []byte("tesla"),
        Val: []byte("$$$"),
        Priority: rand.Int(),
    })
    c.Upsert(&gkvlite.Item{
        Key: []byte("mercedes"),
        Val: []byte("$$"),
        Priority: rand.Int(),
    })
    c.Upsert(&gkvlite.Item{
        Key: []byte("bmw"),
        Val: []byte("$"),
        Priority: rand.Int(),
    })
    
    mercedesItem, err := c.Get("mercedes", true)
    thisIsNil, err := c.Get("the-lunar-rover", true)
    
    c.VisitAscend("ford", func(i *Item) bool {
        // This visitor callback will be invoked with every item
        // with key "ford" and onwards, in key-sorted order.
        // So: "mercedes", "tesla".
        // If we want to stop visiting, return false;
        // otherwise a true return result means keep visiting items.
        return true
    })
    
    err = c.Delete("mercedes")
    mercedesIsNil, err = c.Get("mercedes", true)

Implementation / design
=======================

The fundamental datastructure is an immutable treap.  When used with
random item priorities, treaps have probabilistic balanced tree
behavior with the usual O(log N) performance bounds expected of
balanced binary trees.

The persistence design is append-only, using ideas from Apache CouchDB
/ Couchstore, providing a resilience to process or machine crashes.
On re-opening a file, the implementation scans the file backwards
looking for the last good root record and logically "truncates" the
file.  New mutations proceed from that last good root.  This follows
the "the log is the database" approach of CouchDB / Couchstore /
Couchbase.

The immutable, copy-on-write treap plus the append-only persistence
design allows for easy MVCC snapshotting.
