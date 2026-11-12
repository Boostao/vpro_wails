#!/usr/bin/env Rscript
# Out-of-process R runtime probe for VPro 2.0 (Wave 4B Auto-7)
# Invoked strictly via exec.CommandContext(ctx, rscriptBin, "--vanilla", scriptPath, reqPath, respPath)

args <- commandArgs(trailingOnly = TRUE)
if (length(args) < 2L) {
  stop("Usage: probe_runtime.R <request.json> <response.json>", call. = FALSE)
}
req_path <- args[[1L]]
resp_path <- args[[2L]]

if (!requireNamespace("jsonlite", quietly = TRUE)) {
  # Minimal fallback JSON writer if jsonlite itself is missing
  r_ver <- paste0(R.version$major, ".", R.version$minor)
  out <- sprintf(
    '{"ok":true,"rVersion":"%s","platform":"%s","jsonliteMissing":true,"packages":[]}',
    r_ver, R.version$platform
  )
  writeLines(out, con = resp_path, useBytes = TRUE)
  quit(status = 0L)
}

target_pkgs <- c(
  "vpro", "climr", "vegclust", "vegan", "DBI", "RSQLite",
  "duckdb", "sf", "data.table", "jsonlite", "knitr", "rmarkdown"
)
required_pkgs <- c("vegan", "vegclust", "DBI", "RSQLite", "data.table", "jsonlite", "knitr", "rmarkdown")

pkg_rows <- lapply(target_pkgs, function(pkg) {
  installed <- requireNamespace(pkg, quietly = TRUE)
  ver <- if (installed) as.character(utils::packageVersion(pkg)) else ""
  list(
    name = pkg,
    installed = installed,
    version = ver,
    required = pkg %in% required_pkgs
  )
})

climr_cache_dir <- ""
climr_rasters_cached <- FALSE
if (requireNamespace("climr", quietly = TRUE)) {
  climr_cache_dir <- tryCatch(
    as.character(climr::cache_path()),
    error = function(e) ""
  )
  if (nzchar(climr_cache_dir) && dir.exists(climr_cache_dir)) {
    tif_files <- list.files(climr_cache_dir, pattern = "\\.(tif|tiff|vrt)$", recursive = TRUE, ignore.case = TRUE)
    climr_rasters_cached <- length(tif_files) > 0L
  }
}

resp <- list(
  ok = TRUE,
  rVersion = paste0(R.version$major, ".", R.version$minor),
  rVersionFull = R.version.string,
  platform = R.version$platform,
  libPaths = as.list(.libPaths()),
  climrCachePath = climr_cache_dir,
  climrRastersCached = climr_rasters_cached,
  packages = pkg_rows
)

jsonlite::write_json(resp, path = resp_path, auto_unbox = TRUE, pretty = FALSE, null = "null")
