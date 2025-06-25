func (r *AddonReconciler) reconcile(ctx context.Context, addon *appsv1alpha1.Addon) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// update addon finalizer
	finalizerName := constant.DBResourceFinalizerName
	if controllerutil.AddFinalizer(addon, finalizerName) {
		if err := r.Update(ctx, addon); err != nil {
			return intctrlutil.RequeueWithError(err, logger, "")
		}
	}

	// handle finalizer
	if !addon.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(addon, finalizerName) {
			// delete helm releases
			if err := r.deleteReleases(ctx, addon); err != nil {
				return intctrlutil.RequeueWithError(err, logger, "")
			}
			controllerutil.RemoveFinalizer(addon, finalizerName)
			if err := r.Update(ctx, addon); err != nil {
				return intctrlutil.RequeueWithError(err, logger, "")
			}
		}
		return intctrlutil.Reconciled()
	}

	// check addon dependencies
	if err := r.validateDependencies(ctx, addon); err != nil {
		return intctrlutil.RequeueWithError(err, logger, "")
	}

	// handle addon install
	result, err := r.handleAddonInstall(ctx, addon)
	if err != nil {
		return intctrlutil.RequeueWithError(err, logger, "")
	}
	return result, nil
}

// validateDependencies checks if all dependencies of an addon are installed
func (r *AddonReconciler) validateDependencies(ctx context.Context, addon *appsv1alpha1.Addon) error {
	if len(addon.Spec.Dependencies) == 0 {
		return nil
	}

	logger := log.FromContext(ctx)
	for _, depName := range addon.Spec.Dependencies {
		dep := &appsv1alpha1.Addon{}
		if err := r.Get(ctx, types.NamespacedName{Name: depName}, dep); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "dependency addon not found", "addon", addon.Name, "dependency", depName)
				return fmt.Errorf("dependency addon %s not found", depName)
			}
			return err
		}

		if !addonIsReady(dep) {
			logger.Info("dependency addon not ready", "addon", addon.Name, "dependency", depName)
			return fmt.Errorf("dependency addon %s is not ready", depName)
		}
	}
	return nil
}

// addonIsReady checks if an addon is installed and ready to be depended upon
func addonIsReady(addon *appsv1alpha1.Addon) bool {
	// An addon is considered ready if it's installed and not being deleted
	return addon.Status.Phase == appsv1alpha1.AddonEnabled && addon.DeletionTimestamp.IsZero()
}
