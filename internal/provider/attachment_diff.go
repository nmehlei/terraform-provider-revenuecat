package provider

import "sort"

// diffProductIDs computes the attach and detach lists needed to move from the
// current set of attached products to the desired set.
//
// The provider sends only the difference rather than detaching everything and
// re-attaching: a full detach would briefly revoke entitlement access for live
// customers, which is not an acceptable side effect of a plan that only adds
// one product. Both results are sorted so requests are deterministic.
func diffProductIDs(current, desired []string) (toAttach, toDetach []string) {
	currentSet := make(map[string]struct{}, len(current))
	for _, id := range current {
		currentSet[id] = struct{}{}
	}

	desiredSet := make(map[string]struct{}, len(desired))
	for _, id := range desired {
		desiredSet[id] = struct{}{}
	}

	for id := range desiredSet {
		if _, ok := currentSet[id]; !ok {
			toAttach = append(toAttach, id)
		}
	}
	for id := range currentSet {
		if _, ok := desiredSet[id]; !ok {
			toDetach = append(toDetach, id)
		}
	}

	sort.Strings(toAttach)
	sort.Strings(toDetach)
	return toAttach, toDetach
}

// packageProductSpec is one desired product attachment on a package.
type packageProductSpec struct {
	ProductID           string
	EligibilityCriteria string
}

// diffPackageProducts computes the attach and detach lists for a package.
//
// A product whose eligibility criteria changed is re-attached rather than
// detached and re-attached, since attaching an already-attached product updates
// its criteria in place. Only products dropped from the configuration entirely
// are detached.
func diffPackageProducts(current, desired []packageProductSpec) (toAttach []packageProductSpec, toDetach []string) {
	currentByID := make(map[string]string, len(current))
	for _, entry := range current {
		currentByID[entry.ProductID] = entry.EligibilityCriteria
	}

	desiredByID := make(map[string]struct{}, len(desired))
	for _, entry := range desired {
		desiredByID[entry.ProductID] = struct{}{}

		criteria, attached := currentByID[entry.ProductID]
		if !attached || criteria != entry.EligibilityCriteria {
			toAttach = append(toAttach, entry)
		}
	}

	for _, entry := range current {
		if _, ok := desiredByID[entry.ProductID]; !ok {
			toDetach = append(toDetach, entry.ProductID)
		}
	}

	sort.Slice(toAttach, func(i, j int) bool { return toAttach[i].ProductID < toAttach[j].ProductID })
	sort.Strings(toDetach)
	return toAttach, toDetach
}
