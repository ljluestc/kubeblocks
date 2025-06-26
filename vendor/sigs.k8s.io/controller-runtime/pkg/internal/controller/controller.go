// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the reconcileHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.Queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer c.Queue.Done(obj)

	c.reconcileHandler(context.Background(), obj)
	return true
}

var _ reconcile.Reconciler = &reconcileRecorder{}

// reconcileRecorder implements reconcile.Reconciler.
type reconcileRecorder struct {
	reconcile func(context.Context, reconcile.Request) (reconcile.Result, error)
}

func (r *reconcileRecorder) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	return r.reconcile(ctx, request)
}

// reconcileHandler is a generic function that can handle reconciliation of any resource managed by a Controller.
func (c *Controller) reconcileHandler(ctx context.Context, obj interface{}) {
	// Update metrics after processing each item
	reconcileStartTS := time.Now()
	defer func() {
		c.updateMetrics(time.Since(reconcileStartTS))
	}()

	// Make sure that the object is a valid request.
	req, ok := obj.(reconcile.Request)
	if !ok {
		// As the item in the workqueue is actually invalid, we call
		// Forget here else we'd go into a loop of attempting to
		// process a work item that is invalid.
		c.Queue.Forget(obj)
		c.Log.Error(nil, "Queue item was not a Request", "type", fmt.Sprintf("%T", obj), "value", obj)
		// Return true, don't take a break
		return
	}

	// RunInformersAndControllers the syncHandler, passing it the Namespace/Name string of the
	// resource to be synced.
	result, err := c.Reconcile(ctx, req)
