export namespace guiapp {
	
	export interface ActionView {
	    title: string;
	}
	export interface AppIconView {
	    id: string;
	    label: string;
	    default: boolean;
	    active: boolean;
	}
	export interface PackageStateView {
	    installed: boolean;
	    installAvailable: boolean;
	    uninstallAvailable: boolean;
	    actionable: boolean;
	    actionKind: string;
	    actionUrl: string;
	    mode: string;
	    statusLabel: string;
	    actionLabel: string;
	}
	export interface LinkView {
	    label: string;
	    url: string;
	}
	export interface LicenseView {
	    label: string;
	    url: string;
	    kind: string;
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
	    softwareTypes: string[];
	    availabilityNote: string;
	    openSource: boolean;
	    hasFreeVersion: boolean;
	    license?: LicenseView;
	    externalActionUrl: string;
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
	
	export interface IconSettingsPayload {
	    icons: AppIconView[];
	    activeId: string;
	}
	
	
	

}
