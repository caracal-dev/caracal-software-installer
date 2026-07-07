%global debug_package %{nil}
%undefine _disable_source_fetch
%global upstream_version %{?version_override}%{!?version_override:2.9.0}
%global github_owner %{?github_owner_override}%{!?github_owner_override:caracal-dev}
%global github_repo %{?github_repo_override}%{!?github_repo_override:caracal-software-installer}
%global source_tag %{?source_tag_override}%{!?source_tag_override:v%{upstream_version}}
%global source_dir_name %{github_repo}-%{upstream_version}

Name:           caracal-software-installer
Version:        %{upstream_version}
Release:        %{?release_override}%{!?release_override:1}%{?dist}
Summary:        Catalog-driven installer for optional audio software
License:        MIT
URL:            https://github.com/%{github_owner}/%{github_repo}
Source0:        %{url}/archive/refs/tags/%{source_tag}.tar.gz#/%{name}-%{version}.tar.gz

BuildRequires:  gcc
BuildRequires:  golang >= 1.21
BuildRequires:  glib2-devel
BuildRequires:  gtk3-devel
BuildRequires:  pkgconf-pkg-config
BuildRequires:  webkit2gtk4.1-devel

%description
caracal-software-installer provides a graphical desktop frontend and a
terminal UI for browsing and installing optional DAWs, instruments,
plugins, and audio utilities from a curated catalog.

%prep
%autosetup -n %{source_dir_name}

%build
mkdir -p build
export GOTOOLCHAIN=auto
export GOFLAGS="-buildmode=pie -trimpath -mod=vendor"
go build -tags="desktop,production,webkit2_41" -ldflags="-s -w" -o build/caracal-software-installer-gui .
go build -ldflags="-s -w" -o build/caracal ./cmd/caracal
go build -ldflags="-s -w" -o build/caracal-software-installer ./cmd/caracal-software-installer
go build -ldflags="-s -w" -o build/caracal-download-index ./cmd/caracal-download-index

%check
export GOTOOLCHAIN=auto
export GOFLAGS="-mod=vendor"
go test ./...
scripts/download-index validate

%install
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_prefix}/lib/caracal-software-installer/bin
install -d %{buildroot}%{_prefix}/lib/caracal-software-installer
install -d %{buildroot}%{_datadir}/caracal-software-installer
install -d %{buildroot}%{_datadir}/caracal-software-installer/build/icons
install -d %{buildroot}%{_datadir}/pixmaps

install -pm0755 build/caracal-software-installer %{buildroot}%{_bindir}/caracal-software-installer
install -pm0755 build/caracal-software-installer-gui %{buildroot}%{_bindir}/caracal-software-installer-gui
install -pm0755 build/caracal %{buildroot}%{_bindir}/caracal
install -pm0755 build/caracal-download-index %{buildroot}%{_prefix}/lib/caracal-software-installer/bin/caracal-download-index

cp -a scripts %{buildroot}%{_prefix}/lib/caracal-software-installer/
cp -a data %{buildroot}%{_prefix}/lib/caracal-software-installer/
cp -a assets %{buildroot}%{_datadir}/caracal-software-installer/
install -pm0644 build/icons/*.png %{buildroot}%{_datadir}/caracal-software-installer/build/icons/

install -pm0644 logo.txt %{buildroot}%{_datadir}/caracal-software-installer/logo.txt
install -pm0644 build/appicon.png %{buildroot}%{_datadir}/pixmaps/caracal-software-installer.png
install -Dpm0644 packaging/caracal-software-installer.desktop %{buildroot}%{_datadir}/applications/caracal-software-installer.desktop
install -Dpm0644 packaging/caracal-install-local.desktop %{buildroot}%{_datadir}/applications/caracal-install-local.desktop

%files
%license LICENSE
%doc README.md
%{_bindir}/caracal-software-installer
%{_bindir}/caracal-software-installer-gui
%{_bindir}/caracal
%{_prefix}/lib/caracal-software-installer/bin/caracal-download-index
%{_prefix}/lib/caracal-software-installer/scripts/*
%{_prefix}/lib/caracal-software-installer/data/*
%{_datadir}/caracal-software-installer/logo.txt
%{_datadir}/caracal-software-installer/assets/images
%{_datadir}/caracal-software-installer/assets/screenshots/*
%{_datadir}/caracal-software-installer/build/icons/*
%{_datadir}/pixmaps/caracal-software-installer.png
%{_datadir}/applications/caracal-software-installer.desktop
%{_datadir}/applications/caracal-install-local.desktop

%changelog
* Tue May 05 2026 Atumia <atumia@users.noreply.github.com> - %{version}-%{release}
- Include Wails frontend assets in RPM source archives.
