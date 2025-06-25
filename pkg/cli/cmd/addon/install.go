func install(cmd *cobra.Command, args []string) error {
	if addonInstallOpts.all {
		// install all addons
		return installAllAddons()
	}

	if len(args) == 0 {
		addonList, err := getAddonList(addonInstallOpts.nonDefault)
		if err != nil {
			return err
		}
		selectedAddons, canceled, err := selectAddons(addonList)
		if err != nil {
			return err
		}
		if canceled {
			return nil
		}
		return installAddons(selectedAddons)
	}

	// if args is provided, install the addons
	addonList, err := getAddonList(addonInstallOpts.nonDefault)
	if err != nil {
		return err
	}
	selectedAddons := getSelectedAddons(addonList, args)
	
	// Include dependencies if needed
	if !addonInstallOpts.skipDependencies {
		selectedAddons, err = resolveDependencies(selectedAddons)
		if err != nil {
			return err
		}
	}
	
	return installAddons(selectedAddons)
}

// resolveDependencies finds all dependencies for the selected addons and adds them to the installation list
func resolveDependencies(selectedAddons []string) ([]string, error) {
	client := k8sconfig.GetClientSet()
	addonMap := make(map[string]struct{})
	for _, addon := range selectedAddons {
		addonMap[addon] = struct{}{}
	}
	
	// Process all selected addons and their dependencies
	queue := append([]string{}, selectedAddons...)
	for len(queue) > 0 {
		addonName := queue[0]
		queue = queue[1:]
		
		// Get addon CR to check dependencies
		addon := &appsv1alpha1.Addon{}
		if err := client.Get(context.Background(), types.NamespacedName{Name: addonName}, addon); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("addon %s not found", addonName)
			}
			return nil, err
		}
		
		// Add dependencies to queue if not already included
		for _, dep := range addon.Spec.Dependencies {
			if _, exists := addonMap[dep]; !exists {
				addonMap[dep] = struct{}{}
				queue = append(queue, dep)
				fmt.Printf("Added dependency: %s (required by %s)\n", dep, addonName)
			}
		}
	}
	
	// Convert map back to slice
	result := make([]string, 0, len(addonMap))
	for addon := range addonMap {
		result = append(result, addon)
	}
	
	// Sort dependencies to ensure deterministic installation order
	// This is a simple implementation; a proper topological sort would be more accurate
	return sortAddonsWithDependencies(result)
}

// sortAddonsWithDependencies sorts addons based on their dependencies (simplified)
func sortAddonsWithDependencies(addons []string) ([]string, error) {
	client := k8sconfig.GetClientSet()
	addonDeps := make(map[string][]string)
	
	// Build dependency graph
	for _, addon := range addons {
		addonObj := &appsv1alpha1.Addon{}
		if err := client.Get(context.Background(), types.NamespacedName{Name: addon}, addonObj); err != nil {
			return nil, err
		}
		addonDeps[addon] = addonObj.Spec.Dependencies
	}
	
	// Perform a simple dependency-based sort
	// In a real implementation, this should be a topological sort
	result := make([]string, 0, len(addons))
	visited := make(map[string]bool)
	
	// Helper function for depth-first traversal
	var visit func(string) error
	visit = func(addon string) error {
		if visited[addon] {
			return nil
		}
		
		visited[addon] = true
		for _, dep := range addonDeps[addon] {
			if !contains(addons, dep) {
				continue // Skip dependencies not in our list
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		
		result = append(result, addon)
		return nil
	}
	
	// Visit all addons
	for _, addon := range addons {
		if !visited[addon] {
			if err := visit(addon); err != nil {
				return nil, err
			}
		}
	}
	
	return result, nil
}

// contains checks if a string is in a slice
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
