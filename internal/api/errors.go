package api

import "errors"

var errEbpfNotLoaded = errors.New("ebpf not loaded")
var errProfileCIDROverlap = errors.New("profile cidr overlaps existing profile")
