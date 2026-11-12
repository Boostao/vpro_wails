#!/usr/bin/env Rscript
# Standalone HTML fallback renderer via knitr/rmarkdown when quarto CLI is absent on a workstation

args <- commandArgs(trailingOnly = TRUE)
if (length(args) < 3L) {
  stop("Usage: render_guide_fallback.R <template.qmd> <payload.json> <output.html>", call. = FALSE)
}
qmd_path <- args[[1L]]
payload_path <- args[[2L]]
out_html <- args[[3L]]

# Convert Quarto {=html} fenced block to standard RMarkdown raw HTML if needed and knit
lines <- readLines(qmd_path, warn = FALSE, encoding = "UTF-8")
lines <- gsub("^```\\{=html\\}\\s*$", "", lines)
# Strip closing backticks of raw style block right after </style>
for (i in seq_along(lines)) {
  if (i > 1L && grepl("</style>", lines[[i - 1L]], fixed = TRUE) && grepl("^```\\s*$", lines[[i]])) {
    lines[[i]] <- ""
  }
}
rmd_path <- sub("\\.qmd$", "_fallback.Rmd", qmd_path)
writeLines(lines, rmd_path, useBytes = TRUE)

rmarkdown::render(
  input = rmd_path,
  output_format = rmarkdown::html_document(self_contained = TRUE, theme = NULL),
  output_file = basename(out_html),
  output_dir = dirname(out_html),
  params = list(payload_json = payload_path),
  quiet = TRUE,
  envir = new.env(parent = globalenv())
)
