package arxiv

import "fmt"

func (c *Cache) bindVar(n int) string {
	if c.dbType == DBTypePostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}
