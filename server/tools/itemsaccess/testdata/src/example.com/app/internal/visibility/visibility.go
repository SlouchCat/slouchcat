package visibility

// Scope is a stand-in for the real visibility scope.
type Scope struct{ reason string }

// System returns an unfiltered scope.
func System(reason string) Scope { return Scope{reason: reason} }

// Profile returns a filtered scope.
func Profile(id int64) Scope { return Scope{} }

func internalUse() Scope { return System("allowed inside visibility") }

const ownQuery = "SELECT id FROM items WHERE id IN (SELECT item_id FROM visible_items)"
