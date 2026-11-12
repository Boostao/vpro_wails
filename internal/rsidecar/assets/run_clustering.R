#!/usr/bin/env Rscript
# Out-of-process BEC_R Multivariate Clustering (vegan + vegclust) & Dendrogram Generator
# References:
#   - MoF BEC_R_working/_functions/_cluster_groups.R & _create_dendro.R
#   - vegan::vegdist, stats::hclust, cluster::silhouette, vegclust::vegclust

suppressPackageStartupMessages({
  library(jsonlite)
  library(DBI)
  library(RSQLite)
  library(data.table)
  library(vegan)
  library(vegclust)
})

args <- commandArgs(trailingOnly = TRUE)
if (length(args) < 2L) {
  stop("Usage: run_clustering.R <request.json> <response.json>", call. = FALSE)
}
req_path <- args[[1L]]
resp_path <- args[[2L]]

req <- jsonlite::fromJSON(req_path, simplifyVector = TRUE)
project_path <- as.character(req$projectPath)
project_name <- as.character(req$projectName)
vlists_path <- if (!is.null(req$vlistsPath)) as.character(req$vlistsPath) else ""
method <- if (!is.null(req$method) && nzchar(req$method)) toupper(req$method) else "NC"
distance_metric <- if (!is.null(req$distance) && nzchar(req$distance)) tolower(req$distance) else "bray"
linkage <- if (!is.null(req$linkage) && nzchar(req$linkage)) req$linkage else "ward.D2"
transform_mode <- if (!is.null(req$transform) && nzchar(req$transform)) tolower(req$transform) else "sqrt"
k_req <- if (!is.null(req$k)) as.integer(req$k) else 3L
fuzziness_m <- if (!is.null(req$fuzziness)) as.numeric(req$fuzziness) else 1.25
dnoise_val <- if (!is.null(req$dnoise)) as.numeric(req$dnoise) else 0.75
cut_height <- if (!is.null(req$cutHeight) && is.finite(as.numeric(req$cutHeight)) && as.numeric(req$cutHeight) > 0) as.numeric(req$cutHeight) else NA_real_
apply_lump <- isTRUE(req$applyLump)
lump_table <- if (!is.null(req$lumpTable)) as.character(req$lumpTable) else ""
unit_prefix <- if (!is.null(req$unitPrefix) && nzchar(req$unitPrefix)) as.character(req$unitPrefix) else "CL"

con <- DBI::dbConnect(RSQLite::SQLite(), project_path, flags = RSQLite::SQLITE_RO)
on.exit(DBI::dbDisconnect(con), add = TRUE)

q_ident <- function(x) paste0('"', gsub('"', '""', x, fixed = TRUE), '"')
veg_table <- paste0(project_name, "_Veg")

veg_sql <- sprintf(
  paste(
    'SELECT CAST("PlotNumber" AS TEXT) AS PlotNumber, UPPER(TRIM(CAST("Species" AS TEXT))) AS Species,',
    'COALESCE("Cover1",0) AS c1, COALESCE("Cover2",0) AS c2, COALESCE("Cover3",0) AS c3, COALESCE("TotalA",0) AS ta,',
    'COALESCE("Cover4",0) AS c4, COALESCE("Cover5",0) AS c5, COALESCE("Cover5a",0) AS c5a, COALESCE("Cover5b",0) AS c5b, COALESCE("Cover5c",0) AS c5c, COALESCE("TotalB",0) AS tb,',
    'COALESCE("Cover6",0) AS c6, COALESCE("Cover7",0) AS c7, COALESCE("Cover8",0) AS c8, COALESCE("Cover9",0) AS c9, COALESCE("Cover10",0) AS c10',
    'FROM %s WHERE typeof("PlotNumber")=\'text\' AND TRIM(COALESCE("Species",\'\')) <> \'\''
  ),
  q_ident(veg_table)
)
veg_df <- DBI::dbGetQuery(con, veg_sql)
veg_dt <- data.table::as.data.table(veg_df)

if (!is.null(req$plotNumbers) && length(req$plotNumbers) > 0L) {
  allowed <- as.character(req$plotNumbers)
  veg_dt <- veg_dt[PlotNumber %chin% allowed]
}

if (nrow(veg_dt) == 0L) {
  stop("No vegetation records matched the requested plot scope.", call. = FALSE)
}

# Optional taxonomic lump folding (<Name>_Lump where Use != 0)
lump_applied_count <- 0L
if (apply_lump && nzchar(lump_table) && DBI::dbExistsTable(con, lump_table)) {
  lump_df <- DBI::dbGetQuery(con, sprintf(
    'SELECT UPPER(TRIM(CAST("SppCode" AS TEXT))) AS SppCode, UPPER(TRIM(CAST("LumpCode" AS TEXT))) AS LumpCode, COALESCE("Use", 0) AS UseFlag FROM %s',
    q_ident(lump_table)
  ))
  lump_df <- lump_df[lump_df$UseFlag != 0 & nzchar(lump_df$SppCode) & nzchar(lump_df$LumpCode), , drop = FALSE]
  if (nrow(lump_df) > 0L) {
    lump_map <- stats::setNames(lump_df$LumpCode, lump_df$SppCode)
    matched_idx <- which(veg_dt$Species %in% names(lump_map))
    if (length(matched_idx) > 0L) {
      veg_dt$Species[matched_idx] <- unname(lump_map[veg_dt$Species[matched_idx]])
      lump_applied_count <- length(matched_idx)
    }
  }
}

# Combine cover across strata per (PlotNumber, Species) using Jennings-Barkman complement formula: 100 * (1 - prod(1 - p_i))
combine_pct <- function(vals) {
  v <- pmin(pmax(as.numeric(vals), 0.0), 99.9)
  v <- v[is.finite(v) & v > 0.0]
  if (length(v) == 0L) return(0.0)
  100.0 * (1.0 - prod(1.0 - (v / 100.0)))
}

veg_dt[, tree_cov := ifelse(ta > 0, ta, mapply(function(a, b, c) combine_pct(c(a, b, c)), c1, c2, c3))]
veg_dt[, shrub_cov := ifelse(tb > 0, tb, mapply(function(a, b, c, d, e) combine_pct(c(a, b, c, d, e)), c4, c5, c5a, c5b, c5c))]
veg_dt[, row_cov := mapply(function(tr, sh, c6v, c7v, c8v, c9v, c10v) {
  combine_pct(c(tr, sh, c6v, c7v, c8v, c9v, c10v))
}, tree_cov, shrub_cov, c6, c7, c8, c9, c10)]

veg_agg <- veg_dt[row_cov > 0, .(Cover = combine_pct(row_cov)), by = .(PlotNumber, Species)]
plots_unique <- sort(unique(veg_agg$PlotNumber))
spp_unique <- sort(unique(veg_agg$Species))

if (length(plots_unique) < 3L) {
  stop(sprintf("Clustering requires at least 3 vegetated plots in scope (found %d).", length(plots_unique)), call. = FALSE)
}

k <- max(2L, min(k_req, length(plots_unique) - 1L))

# Construct dense Plot x Species community matrix
mat_raw <- matrix(0.0, nrow = length(plots_unique), ncol = length(spp_unique),
                  dimnames = list(plots_unique, spp_unique))
row_idx <- match(veg_agg$PlotNumber, plots_unique)
col_idx <- match(veg_agg$Species, spp_unique)
mat_raw[cbind(row_idx, col_idx)] <- veg_agg$Cover

# Abundance transformation
mat_trans <- switch(transform_mode,
  "sqrt" = sqrt(mat_raw),
  "log1p" = log1p(mat_raw),
  "chord" = vegan::decostand(mat_raw, method = "normalize"),
  "hellinger" = vegan::decostand(mat_raw, method = "hellinger"),
  mat_raw
)

# Hierarchical clustering & dendrogram (always computed so interactive BEC dendrogram is available!)
dist_obj <- if (identical(distance_metric, "chord")) {
  stats::dist(vegan::decostand(mat_trans, method = "normalize"), method = "euclidean")
} else if (distance_metric %in% c("bray", "jaccard", "kulczynski", "euclidean", "manhattan")) {
  vegan::vegdist(mat_trans, method = distance_metric)
} else {
  vegan::vegdist(mat_trans, method = "bray")
}

hc_method <- if (linkage %in% c("ward.D2", "average", "complete", "single", "mcquitty")) linkage else "ward.D2"
hc <- stats::hclust(dist_obj, method = hc_method)
coph_corr <- suppressWarnings(stats::cor(dist_obj, stats::cophenetic(hc)))
if (!is.finite(coph_corr)) coph_corr <- 0.0

# Extract rectangular dendrogram segments & ordered leaf positions (pure base R over hclust)
build_dendrogram_geometry <- function(hc_obj) {
  n <- length(hc_obj$order)
  leaf_pos <- numeric(n)
  leaf_pos[hc_obj$order] <- seq_len(n)

  node_x <- numeric(n - 1L)
  node_y <- as.numeric(hc_obj$height)
  segments <- vector("list", 3L * (n - 1L))
  seg_k <- 0L

  get_coord <- function(idx) {
    if (idx < 0L) {
      c(leaf_pos[[-idx]], 0.0)
    } else {
      c(node_x[[idx]], node_y[[idx]])
    }
  }

  for (i in seq_len(n - 1L)) {
    left <- get_coord(hc_obj$merge[i, 1L])
    right <- get_coord(hc_obj$merge[i, 2L])
    top_y <- node_y[[i]]
    mid_x <- 0.5 * (left[[1L]] + right[[1L]])
    node_x[[i]] <- mid_x

    # Left vertical stem, horizontal bridge, right vertical stem
    seg_k <- seg_k + 1L
    segments[[seg_k]] <- list(x1 = left[[1L]], y1 = left[[2L]], x2 = left[[1L]], y2 = top_y)
    seg_k <- seg_k + 1L
    segments[[seg_k]] <- list(x1 = left[[1L]], y1 = top_y, x2 = right[[1L]], y2 = top_y)
    seg_k <- seg_k + 1L
    segments[[seg_k]] <- list(x1 = right[[1L]], y1 = right[[2L]], x2 = right[[1L]], y2 = top_y)
  }

  leaves <- lapply(seq_len(n), function(pos_i) {
    orig_idx <- hc_obj$order[[pos_i]]
    list(
      plotNumber = hc_obj$labels[[orig_idx]],
      x = as.numeric(pos_i),
      order = as.integer(pos_i)
    )
  })

  # Build Newick tree representation
  to_newick <- function(idx, parent_h) {
    if (idx < 0L) {
      lbl <- gsub("[:(),;[:space:]]", "_", hc_obj$labels[[-idx]])
      br <- max(0.0, round(parent_h, 4))
      return(sprintf("%s:%.4f", lbl, br))
    }
    h_curr <- node_y[[idx]]
    l_str <- to_newick(hc_obj$merge[idx, 1L], h_curr)
    r_str <- to_newick(hc_obj$merge[idx, 2L], h_curr)
    if (is.na(parent_h)) {
      sprintf("(%s,%s);", l_str, r_str)
    } else {
      sprintf("(%s,%s):%.4f", l_str, r_str, max(0.0, round(parent_h - h_curr, 4)))
    }
  }
  newick_str <- to_newick(n - 1L, NA_real_)

  list(
    maxHeight = round(max(c(0.0001, node_y)), 4),
    segments = segments,
    leaves = leaves,
    newick = newick_str
  )
}

dendro <- build_dendrogram_geometry(hc)

# Execute requested clustering algorithm: vegclust ("NC" or "KM") or Hierarchical ("HCLUST")
cluster_labels <- character(length(plots_unique))
names(cluster_labels) <- plots_unique
max_membership <- rep(1.0, length(plots_unique))
is_noise <- rep(FALSE, length(plots_unique))
effective_cut <- if (is.finite(cut_height)) cut_height else round(0.55 * dendro$maxHeight, 4)

set.seed(42L)
if (method %in% c("NC", "KM", "FCM")) {
  vc_method <- if (identical(method, "NC")) "NC" else if (identical(method, "FCM")) "FCM" else "KM"
  vc_data <- vegan::decostand(mat_trans, method = "normalize")
  vc_fit <- if (identical(vc_method, "NC")) {
    vegclust::vegclust(
      x = vc_data,
      mobileCenters = k,
      method = "NC",
      m = max(1.05, fuzziness_m),
      dnoise = max(0.05, dnoise_val),
      nstart = 5L,
      iter.max = 100L
    )
  } else if (identical(vc_method, "FCM")) {
    vegclust::vegclust(
      x = vc_data,
      mobileCenters = k,
      method = "FCM",
      m = max(1.05, fuzziness_m),
      nstart = 5L,
      iter.max = 100L
    )
  } else {
    vegclust::vegclust(
      x = vc_data,
      mobileCenters = k,
      method = "KM",
      nstart = 5L,
      iter.max = 100L
    )
  }
  memb_mat <- as.matrix(vc_fit$memb)
  defuzz <- vegclust::defuzzify(vc_fit)$cluster
  for (i in seq_along(plots_unique)) {
    raw_cl <- as.character(defuzz[[i]])
    max_membership[[i]] <- round(max(memb_mat[i, ], na.rm = TRUE), 4)
    if (is.na(raw_cl) || identical(raw_cl, "N") || identical(raw_cl, "N+")) {
      is_noise[[i]] <- TRUE
      cluster_labels[[i]] <- paste0(unit_prefix, "_Noise")
    } else {
      num_part <- suppressWarnings(as.integer(gsub("[^0-9]", "", raw_cl)))
      if (is.finite(num_part)) {
        cluster_labels[[i]] <- sprintf("%s_%02d", unit_prefix, num_part)
      } else {
        cluster_labels[[i]] <- paste0(unit_prefix, "_", raw_cl)
      }
    }
  }
} else {
  grp <- if (is.finite(cut_height)) {
    stats::cutree(hc, h = cut_height)
  } else {
    if (k >= 2L && k <= length(plots_unique)) {
      # Approximate cut height halfway between (n-k) and (n-k+1) merge heights
      n_m <- nrow(hc$merge)
      idx_lo <- max(1L, n_m - k + 1L)
      idx_hi <- min(n_m, n_m - k + 2L)
      effective_cut <- round(0.5 * (hc$height[[idx_lo]] + hc$height[[idx_hi]]), 4)
    }
    stats::cutree(hc, k = k)
  }
  for (i in seq_along(plots_unique)) {
    cluster_labels[[i]] <- sprintf("%s_%02d", unit_prefix, as.integer(grp[[i]]))
  }
}

# Compute silhouette widths across non-noise clusters when >= 2 clusters exist
sil_widths <- rep(0.0, length(plots_unique))
mean_sil <- 0.0
int_codes <- as.integer(factor(cluster_labels))
if (length(unique(int_codes)) >= 2L && length(unique(int_codes)) < length(plots_unique)) {
  if (requireNamespace("cluster", quietly = TRUE)) {
    sil_obj <- tryCatch(cluster::silhouette(int_codes, dist_obj), error = function(e) NULL)
    if (!is.null(sil_obj)) {
      sil_mat <- as.matrix(sil_obj)
      sil_widths <- round(as.numeric(sil_mat[, "sil_width"]), 4)
      mean_sil <- round(mean(sil_widths, na.rm = TRUE), 4)
    }
  }
}

# Attach ScientificName / EnglishName from VLists.db if provided
spp_meta <- new.env(hash = TRUE, parent = emptyenv())
if (nzchar(vlists_path) && file.exists(vlists_path)) {
  vcon <- tryCatch(DBI::dbConnect(RSQLite::SQLite(), vlists_path, flags = RSQLite::SQLITE_RO), error = function(e) NULL)
  if (!is.null(vcon)) {
    on.exit(DBI::dbDisconnect(vcon), add = TRUE)
    if (DBI::dbExistsTable(vcon, "USysAllSpecs")) {
      srows <- DBI::dbGetQuery(vcon,
        'SELECT UPPER(TRIM(CAST("Code" AS TEXT))) AS Code, CAST("ScientificName" AS TEXT) AS ScientificName, CAST("EnglishName" AS TEXT) AS EnglishName, COALESCE("Lifeform", 0) AS Lifeform FROM "USysAllSpecs" WHERE COALESCE("Codetype",\'U\') IN (\'U\',\'D\')'
      )
      srows <- srows[!duplicated(srows$Code), , drop = FALSE]
      for (i in seq_len(nrow(srows))) {
        assign(srows$Code[[i]], list(
          scientificName = ifelse(is.na(srows$ScientificName[[i]]), "", srows$ScientificName[[i]]),
          englishName = ifelse(is.na(srows$EnglishName[[i]]), "", srows$EnglishName[[i]]),
          lifeform = as.integer(srows$Lifeform[[i]])
        ), envir = spp_meta)
      }
    }
  }
}

# Compute Dufrêne-Legendre Indicator Value (IndVal = Specificity * Fidelity) & Cluster Summaries
unique_clusters <- sort(unique(cluster_labels))
non_noise_clusters <- unique_clusters[!grepl("_Noise$", unique_clusters)]
if (length(non_noise_clusters) == 0L) non_noise_clusters <- unique_clusters

# Mean abundance of each species per cluster for specificity
cl_mean_mat <- matrix(0.0, nrow = length(non_noise_clusters), ncol = ncol(mat_raw),
                      dimnames = list(non_noise_clusters, colnames(mat_raw)))
for (ci in seq_along(non_noise_clusters)) {
  cname <- non_noise_clusters[[ci]]
  idx <- which(cluster_labels == cname)
  cl_mean_mat[ci, ] <- colMeans(mat_raw[idx, , drop = FALSE])
}
col_sum_means <- colSums(cl_mean_mat)

cluster_summaries <- lapply(unique_clusters, function(cname) {
  idx <- which(cluster_labels == cname)
  n_pl <- length(idx)
  sub_m <- mat_raw[idx, , drop = FALSE]
  pres_pct <- (colSums(sub_m > 0) / n_pl) * 100.0
  mean_cov <- colMeans(sub_m)

  if (cname %in% non_noise_clusters) {
    ci <- match(cname, non_noise_clusters)
    specificity <- ifelse(col_sum_means > 0, cl_mean_mat[ci, ] / col_sum_means, 0.0)
  } else {
    specificity <- rep(0.0, ncol(mat_raw))
  }
  fidelity <- pres_pct / 100.0
  indval <- specificity * fidelity

  ord <- order(-indval, -pres_pct, -mean_cov)
  ord <- ord[pres_pct[ord] > 0]
  top_ord <- head(ord, 8L)

  indicators <- lapply(top_ord, function(si) {
    scode <- colnames(mat_raw)[[si]]
    meta <- if (exists(scode, envir = spp_meta, inherits = FALSE)) get(scode, envir = spp_meta) else list(scientificName = "", englishName = "", lifeform = 0L)
    present_vals <- sub_m[sub_m[, si] > 0, si]
    list(
      species = scode,
      scientificName = meta$scientificName,
      englishName = meta$englishName,
      lifeform = meta$lifeform,
      constancyPct = round(pres_pct[[si]], 1),
      meanCover = round(mean(present_vals), 2),
      indVal = round(indval[[si]] * 100.0, 1)
    )
  })

  list(
    clusterUnit = cname,
    isNoise = grepl("_Noise$", cname),
    plotCount = as.integer(n_pl),
    meanRichness = round(mean(rowSums(sub_m > 0)), 1),
    meanSilhouette = round(mean(sil_widths[idx], na.rm = TRUE), 4),
    indicators = indicators
  )
})

# Decorate dendrogram leaves with assigned cluster unit
for (li in seq_along(dendro$leaves)) {
  pnum <- dendro$leaves[[li]]$plotNumber
  dendro$leaves[[li]]$clusterUnit <- unname(cluster_labels[[pnum]])
  dendro$leaves[[li]]$isNoise <- isTRUE(unname(is_noise[[match(pnum, plots_unique)]]))
}
dendro$cutHeight <- effective_cut

memberships <- lapply(seq_along(plots_unique), function(i) {
  pnum <- plots_unique[[i]]
  list(
    plotNumber = pnum,
    clusterUnit = unname(cluster_labels[[i]]),
    membership = max_membership[[i]],
    isNoise = is_noise[[i]],
    silhouette = sil_widths[[i]],
    speciesRichness = as.integer(sum(mat_raw[i, ] > 0))
  )
})

resp <- list(
  ok = TRUE,
  method = method,
  distance = distance_metric,
  linkage = hc_method,
  transform = transform_mode,
  k = as.integer(k),
  plotCount = as.integer(length(plots_unique)),
  speciesCount = as.integer(length(spp_unique)),
  lumpAppliedRows = as.integer(lump_applied_count),
  copheneticCorrelation = round(coph_corr, 4),
  meanSilhouette = mean_sil,
  noiseCount = as.integer(sum(is_noise)),
  clusters = cluster_summaries,
  memberships = memberships,
  dendrogram = dendro
)

jsonlite::write_json(resp, path = resp_path, auto_unbox = TRUE, pretty = FALSE, null = "null")
