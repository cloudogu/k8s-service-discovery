package types

// Category categories multiple entries in the warp menu
type Category struct {
	Title   string
	Order   int
	Entries Entries
}

func (c Category) String() string {
	return c.Title
}

// Categories collection of warp Categories
type Categories []*Category

func (c Categories) Len() int {
	return len(c)
}

func (c Categories) Less(i, j int) bool {
	if c[i].Order == c[j].Order {
		return c[i].Title < c[j].Title
	}
	return c[i].Order > c[j].Order
}

func (c Categories) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// InsertCategories adds new categories to the slice.
func (c *Categories) InsertCategories(newCategories Categories) {
	for _, newCategory := range newCategories {
		c.InsertCategory(newCategory)
	}
}

// InsertCategory adds a new category to the slice. If the title are same the entries will be merged.
func (c *Categories) InsertCategory(newCategory *Category) {
	for _, category := range *c {
		if category.Title == newCategory.Title {
			category.Entries = append(category.Entries, newCategory.Entries...)
			return
		}
	}
	*c = append(*c, newCategory)
}
