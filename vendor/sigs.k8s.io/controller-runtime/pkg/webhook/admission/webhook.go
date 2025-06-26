func (wh *Webhook) Handle(ctx context.Context, req Request) (response Response) {
	if wh.RecoverPanic {
		defer func() {
			if r := recover(); r != nil {
				for _, fn := range utilruntime.PanicHandlers {
					fn(ctx, r)
				}
				response = Errored(http.StatusInternalServerError, fmt.Errorf("panic: %v [recovered]", r))
				return
			}
		}()
	}

	reqLog := wh.getLogger(&req)
	ctx = logf.IntoContext(ctx, reqLog)
