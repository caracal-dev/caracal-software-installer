export namespace guiapp {
	
	export interface ActionView {
	    title: string;
	}
	export interface PackageStateView {
	    installed: boolean;
	    installAvailable: boolean;
	    uninstallAvailable: boolean;
	    actionable: boolean;
	    mode: string;
	    statusLabel: string;
	    actionLabel: string;
	}
	export interface LinkView {
	    label: string;
	    url: string;
	}
	export interface PackageView {
	    id: string;
	    name: string;
	    vendor: string;
	    categoryName: string;
	    subcategoryName: string;
	    summary: string;
	    description: string;
	    notes: string[];
	    links: LinkView[];
	    availabilityNote: string;
	    installActions: ActionView[];
	    uninstallActions: ActionView[];
	    state: PackageStateView;
	}
	export interface SubcategoryView {
	    id: string;
	    name: string;
	    description: string;
	    packages: PackageView[];
	}
	export interface CategoryView {
	    id: string;
	    name: string;
	    description: string;
	    accent: string;
	    subcategories: SubcategoryView[];
	}
	export interface CatalogPayload {
	    logo: string;
	    categories: CategoryView[];
	}
	
	
	
	

}

