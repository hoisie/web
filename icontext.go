package web

type IContext interface {
	Create(c *Context) (newContext IContext)
	BeforeHandler() (continueToHandler bool)
}
