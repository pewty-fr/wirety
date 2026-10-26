package ipam

import "errors"

// ErrNoAddressAvailable is returned by Repository.AcquireIP when every address
// of the prefix is taken.
var ErrNoAddressAvailable = errors.New("no address available in prefix")
