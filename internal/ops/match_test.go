package ops

import "testing"

func TestMatchSubset(t *testing.T) {
	item := map[string]any{"name": "prod", "host": "db.example.com", "port": int64(5432)}

	t.Run("matches when item has all where keys", func(t *testing.T) {
		where := map[string]any{"name": "prod"}
		if !Matches(item, where, MatchSubset, 0, 1) {
			t.Error("expected match")
		}
	})

	t.Run("does not match on missing key", func(t *testing.T) {
		where := map[string]any{"region": "us"}
		if Matches(item, where, MatchSubset, 0, 1) {
			t.Error("expected no match")
		}
	})

	t.Run("does not match on different value", func(t *testing.T) {
		where := map[string]any{"name": "staging"}
		if Matches(item, where, MatchSubset, 0, 1) {
			t.Error("expected no match")
		}
	})

	t.Run("matches with extra keys in item", func(t *testing.T) {
		where := map[string]any{"name": "prod", "host": "db.example.com"}
		if !Matches(item, where, MatchSubset, 0, 1) {
			t.Error("expected match")
		}
	})

	t.Run("default mode behaves like subset", func(t *testing.T) {
		where := map[string]any{"name": "prod"}
		if !Matches(item, where, "", 0, 1) {
			t.Error("expected match with empty mode (default=subset)")
		}
	})
}

func TestMatchExact(t *testing.T) {
	item := map[string]any{"name": "prod", "port": int64(5432)}

	t.Run("matches with exactly same keys", func(t *testing.T) {
		where := map[string]any{"name": "prod", "port": int64(5432)}
		if !Matches(item, where, MatchExact, 0, 1) {
			t.Error("expected match")
		}
	})

	t.Run("fails with extra keys in item", func(t *testing.T) {
		where := map[string]any{"name": "prod"}
		if Matches(item, where, MatchExact, 0, 1) {
			t.Error("expected no match when item has extra keys")
		}
	})

	t.Run("fails with missing keys in item", func(t *testing.T) {
		where := map[string]any{"name": "prod", "port": int64(5432), "host": "localhost"}
		if Matches(item, where, MatchExact, 0, 1) {
			t.Error("expected no match when where has extra keys")
		}
	})
}

func TestMatchAll(t *testing.T) {
	t.Run("always matches", func(t *testing.T) {
		item := map[string]any{"anything": "value"}
		if !Matches(item, nil, MatchAll, 0, 1) {
			t.Error("expected match")
		}
	})

	t.Run("matches empty item", func(t *testing.T) {
		item := map[string]any{}
		if !Matches(item, nil, MatchAll, 0, 1) {
			t.Error("expected match")
		}
	})
}

func TestMatchIndex(t *testing.T) {
	item := map[string]any{"name": "a"}

	t.Run("positive index matches", func(t *testing.T) {
		if !Matches(item, int64(2), MatchIndex, 2, 5) {
			t.Error("expected match at index 2")
		}
	})

	t.Run("positive index does not match other", func(t *testing.T) {
		if Matches(item, int64(2), MatchIndex, 3, 5) {
			t.Error("expected no match at index 3")
		}
	})

	t.Run("negative index -1 matches last", func(t *testing.T) {
		if !Matches(item, int64(-1), MatchIndex, 4, 5) {
			t.Error("expected match at last index")
		}
	})

	t.Run("negative index -2 matches second to last", func(t *testing.T) {
		if !Matches(item, int64(-2), MatchIndex, 3, 5) {
			t.Error("expected match at second-to-last index")
		}
	})

	t.Run("out of range does not match", func(t *testing.T) {
		if Matches(item, int64(10), MatchIndex, 0, 5) {
			t.Error("expected no match for out-of-range index")
		}
	})

	t.Run("negative out of range does not match", func(t *testing.T) {
		if Matches(item, int64(-10), MatchIndex, 0, 5) {
			t.Error("expected no match for negative out-of-range index")
		}
	})
}

func TestMatchHasKey(t *testing.T) {
	item := map[string]any{"name": "prod", "port": int64(5432)}

	t.Run("key present", func(t *testing.T) {
		if !Matches(item, "name", MatchHasKey, 0, 1) {
			t.Error("expected match")
		}
	})

	t.Run("key absent", func(t *testing.T) {
		if Matches(item, "host", MatchHasKey, 0, 1) {
			t.Error("expected no match")
		}
	})
}

func TestMatchNotHasKey(t *testing.T) {
	item := map[string]any{"name": "prod", "port": int64(5432)}

	t.Run("key present", func(t *testing.T) {
		if Matches(item, "name", MatchNotHasKey, 0, 1) {
			t.Error("expected no match")
		}
	})

	t.Run("key absent", func(t *testing.T) {
		if !Matches(item, "host", MatchNotHasKey, 0, 1) {
			t.Error("expected match")
		}
	})
}

func TestMatchRegex(t *testing.T) {
	t.Run("string pattern matches", func(t *testing.T) {
		item := map[string]any{"name": "prod-db-01"}
		where := map[string]any{"name": "prod-.*"}
		if !Matches(item, where, MatchRegex, 0, 1) {
			t.Error("expected match")
		}
	})

	t.Run("string pattern does not match", func(t *testing.T) {
		item := map[string]any{"name": "staging-db-01"}
		where := map[string]any{"name": "prod-.*"}
		if Matches(item, where, MatchRegex, 0, 1) {
			t.Error("expected no match")
		}
	})

	t.Run("non-string item value fails", func(t *testing.T) {
		item := map[string]any{"port": int64(5432)}
		where := map[string]any{"port": "54.."}
		if Matches(item, where, MatchRegex, 0, 1) {
			t.Error("expected no match when item value is non-string")
		}
	})

	t.Run("multiple patterns all must match", func(t *testing.T) {
		item := map[string]any{"name": "prod-db", "env": "production"}
		where := map[string]any{"name": "prod-.*", "env": "prod.*"}
		if !Matches(item, where, MatchRegex, 0, 1) {
			t.Error("expected match")
		}
	})

	t.Run("one pattern fails means no match", func(t *testing.T) {
		item := map[string]any{"name": "prod-db", "env": "staging"}
		where := map[string]any{"name": "prod-.*", "env": "prod.*"}
		if Matches(item, where, MatchRegex, 0, 1) {
			t.Error("expected no match")
		}
	})

	t.Run("non-string where value compared literally", func(t *testing.T) {
		item := map[string]any{"name": "prod", "port": int64(5432)}
		where := map[string]any{"name": "prod", "port": int64(5432)}
		if !Matches(item, where, MatchRegex, 0, 1) {
			t.Error("expected match for non-string where values compared literally")
		}
	})

	t.Run("anchored pattern is full match", func(t *testing.T) {
		item := map[string]any{"name": "prod-db"}
		where := map[string]any{"name": "prod"}
		if Matches(item, where, MatchRegex, 0, 1) {
			t.Error("expected no match since full match is required")
		}
	})
}
