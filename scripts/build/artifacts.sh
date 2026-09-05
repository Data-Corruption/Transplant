# Local build and packaging operations.
#
# This file is sourced by ../build.sh. It owns frontend generation, Go builds,
# build verification, deterministic installer rendering, and release packaging.


# make_ldflags <devMode>
# Prints the -ldflags string for a build with the given devMode value.
make_ldflags() {
  local pkg
  pkg="$(go list -m)/internal/build"
  local ldflags="-X '${pkg}.name=$APP_NAME'"
  ldflags+=" -X '${pkg}.version=$VERSION'"
  ldflags+=" -X '${pkg}.contactURL=$CONTACT_URL'"
  ldflags+=" -X '${pkg}.defaultLogLevel=$DEFAULT_LOG_LEVEL'"
  ldflags+=" -X '${pkg}.serviceEnabled=$SERVICE_ENABLED'"
  ldflags+=" -X '${pkg}.serviceDesc=$SERVICE_DESC'"
  ldflags+=" -X '${pkg}.serviceArgs=$SERVICE_ARGS'"
  ldflags+=" -X '${pkg}.serviceDefaultPort=$SERVICE_DEFAULT_PORT'"
  ldflags+=" -X '${pkg}.certIdentity=$CERT_IDENTITY'"
  ldflags+=" -X '${pkg}.oidcIssuer=$OIDC_ISSUER'"
  ldflags+=" -X '${pkg}.devMode=$1'"
  printf '%s' "$ldflags"
}

go_build() {
  local ldflags
  ldflags=$(make_ldflags "$DEV_MODE")

  BUILD_OUTS=()
  VERIFY_BUILD_OUT="$OUT_DIR/linux-$HOST_GOARCH"

  local targets=("linux-$HOST_GOARCH")
  if [[ "$BUILD_KIND" == "prod-all" ]]; then
    targets=(linux-amd64 linux-arm64 windows-amd64 windows-arm64)
  fi

  # Pure Go (no cgo): every target cross-compiles with GOOS/GOARCH alone, and
  # Linux binaries are fully static by default - they must run on any distro,
  # including NixOS which has no /lib64/ld-linux-*.so glibc loader.
  local target build_out goos goarch
  for target in "${targets[@]}"; do
    goos="${target%%-*}"
    goarch="${target##*-}"
    build_out="$OUT_DIR/$target"
    [[ "$goos" == "windows" ]] && build_out+=".exe"

    GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
      go build -trimpath -buildvcs=false \
      -ldflags="$ldflags" -o "$build_out" "$GO_MAIN_PATH"
    BUILD_OUTS+=("$build_out")
    printf "🟢 Built %s\n" "$build_out"
  done
}

verify_build() {
  # Linux release binaries must be fully static (pure Go, no cgo - see
  # go_build). For static binaries ldd prints "not a dynamic executable" (or
  # "statically linked" on some systems); any other output line means dynamic
  # linking crept back in (e.g. a new dependency pulled in cgo).
  if ldd "$VERIFY_BUILD_OUT" 2>&1 | grep -Eqv 'not a dynamic executable|statically linked'; then
    printf "🔴 Error: %s is dynamically linked:\n" "$VERIFY_BUILD_OUT"
    ldd "$VERIFY_BUILD_OUT" 2>&1 || true
    exit 1
  fi
  printf "🟢 Verified static linking\n"

  # Only verify the host-arch Linux binary (the only one that can run here).
  BUILD_VARS=$("$VERIFY_BUILD_OUT" --build-vars)
  export BUILD_VARS

  check_var "name" "$APP_NAME"
  check_var "version" "$VERSION"
  check_var "contactURL" "$CONTACT_URL"
  check_var "defaultLogLevel" "$DEFAULT_LOG_LEVEL"
  check_var "serviceEnabled" "$SERVICE_ENABLED"
  check_var "serviceDesc" "$SERVICE_DESC"
  check_var "serviceArgs" "$SERVICE_ARGS"
  check_var "serviceDefaultPort" "$SERVICE_DEFAULT_PORT"
  check_var "devMode" "$DEV_MODE"
  if [[ "$MODE" == "ci" ]]; then
    check_var "certIdentity" "$CERT_IDENTITY"
    check_var "oidcIssuer" "$OIDC_ISSUER"
  fi

  printf "🟢 Build variables verified\n"
}

# sed_replacement <value>
# Escapes a value for use as the replacement side of an s|pattern|value|
# expression: backslash and ampersand are special to sed there, and the pipe
# is the delimiter. Free-text values such as SERVICE_DESC would otherwise
# corrupt the rendered installer silently.
sed_replacement() {
  printf '%s' "$1" | sed -e 's/[\\&|]/\\&/g'
}

# render_installer <template> <output>
# Renders either installer template from the same build variable source and
# rejects incomplete output before it can be packaged or tested.
render_installer() {
  local template="$1"
  local output="$2"
  sed -e "s|<APP_NAME>|$(sed_replacement "$APP_NAME")|g" \
      -e "s|<RELEASE_URL>|$(sed_replacement "$RELEASE_URL")|g" \
      -e "s|<SERVICE>|$(sed_replacement "$SERVICE_ENABLED")|g" \
      -e "s|<SERVICE_DESC>|$(sed_replacement "$SERVICE_DESC")|g" \
      -e "s|<SERVICE_ARGS>|$(sed_replacement "$SERVICE_ARGS")|g" \
      -e "s|<CERT_IDENTITY>|$(sed_replacement "$CERT_IDENTITY")|g" \
      -e "s|<OIDC_ISSUER>|$(sed_replacement "$OIDC_ISSUER")|g" \
      -e "s|<COSIGN_VERSION>|$(sed_replacement "$COSIGN_VERSION")|g" \
      -e "s|<COSIGN_SHA_LINUX_AMD64>|$(sed_replacement "$COSIGN_SHA_LINUX_AMD64")|g" \
      -e "s|<COSIGN_SHA_LINUX_ARM64>|$(sed_replacement "$COSIGN_SHA_LINUX_ARM64")|g" \
      -e "s|<COSIGN_SHA_WINDOWS_AMD64>|$(sed_replacement "$COSIGN_SHA_WINDOWS_AMD64")|g" \
      "$template" > "$output"

  if grep -Eq '<[A-Z][A-Z0-9_]*>' "$output"; then
    printf "🔴 Unrendered placeholder in %s\n" "$output" >&2
    return 1
  fi
}

package_installers() {
  mkdir -p "$RELEASE_DIR"

  render_installer "./scripts/install.sh" "$RELEASE_DIR/install.sh"
  printf "🟢 Processed install.sh\n"

  render_installer "./scripts/install.ps1" "$RELEASE_DIR/install.ps1"
  printf "🟢 Processed install.ps1\n"
}

package_binaries() {
  mkdir -p "$VERSION_DIR"

  local build_out gzip_out
  for build_out in "${BUILD_OUTS[@]}"; do
    gzip_out="$VERSION_DIR/$(basename "$build_out").gz"
    gzip -c -n -- "$build_out" > "$gzip_out"
    printf "🟢 Gzipped %s\n" "$build_out"
  done
}

write_release_version() {
  mkdir -p "$VERSION_DIR"
  printf '%s\n' "$VERSION" > "$VERSION_DIR/version"
  printf "🟢 Release packaged in %s\n" "$VERSION_DIR"
}

# One file listing the sha256 of every release artifact (gzipped binaries +
# version marker). Each installer verifies its cosign signature once, then
# plain sha256-matches the selected artifact against it.
generate_checksums() {
  (
    cd "$VERSION_DIR" || exit 1
    sha256sum linux-*.gz windows-*.gz version > checksums.txt
  )
  printf "🟢 Generated checksums.txt\n"
}

sign_application_release() {
  run_step "Signed checksums.txt" "Failed to sign checksums.txt" \
    "$COSIGN_BIN" sign-blob --yes --bundle "$VERSION_DIR/checksums.txt.cosign.bundle" "$VERSION_DIR/checksums.txt"
}
