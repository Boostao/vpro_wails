#!/usr/bin/env Rscript
# Out-of-process Plot Climate (climr + offline BGC lapse fallback) & Ministry CMD -> aSMR calculator
# References:
#   - MoF BEC_R_working/_functions/_get_plot_climr_data.R
#   - MoF BEC_R_working/_functions/_calculate_aSMR.R
#   - MoF aSMR-x-rSMR/aSMR_Rules_HalfStep.csv & CMD_adjustments.csv

suppressPackageStartupMessages({
  library(jsonlite)
  library(data.table)
  library(DBI)
  library(RSQLite)
})

args <- commandArgs(trailingOnly = TRUE)
if (length(args) < 2L) {
  stop("Usage: compute_climate_asmr.R <request.json> <response.json>", call. = FALSE)
}
req_path <- args[[1L]]
resp_path <- args[[2L]]

req <- jsonlite::fromJSON(req_path, simplifyVector = TRUE)
assets_dir <- req$assetsDir
ref_period <- if (!is.null(req$refPeriod) && nzchar(req$refPeriod)) req$refPeriod else "1961_1990"

rules_dt <- data.table::fread(file.path(assets_dir, "aSMR_Rules_HalfStep.csv"))
cmd_adj_dt <- data.table::fread(file.path(assets_dir, "CMD_adjustments.csv"))
bgc_norm_dt <- data.table::fread(file.path(assets_dir, "bgc_climate_normals.csv"))

# Validate project database compatibility via vpro package if installed
vpro_compatible <- FALSE
if (!is.null(req$projectPath) && nzchar(req$projectPath) &&
    !is.null(req$projectName) && nzchar(req$projectName) &&
    requireNamespace("vpro", quietly = TRUE)) {
  vpro_compatible <- tryCatch({
    insp <- vpro::vpro_project_inspect(req$projectPath, req$projectName)
    isTRUE(insp$compatible)
  }, error = function(e) FALSE)
}

# Check whether local climr cache rasters are available on disk
can_use_climr_local <- FALSE
if (requireNamespace("climr", quietly = TRUE) && !isTRUE(req$forceOfflineModel)) {
  cache_dir <- tryCatch(as.character(climr::cache_path()), error = function(e) "")
  if (nzchar(cache_dir) && dir.exists(cache_dir)) {
    tifs <- list.files(cache_dir, pattern = "\\.(tif|tiff|vrt)$", recursive = TRUE, ignore.case = TRUE)
    can_use_climr_local <- length(tifs) > 0L
  }
}

# Compute climate normals for uncached coordinate points
compute_bgc_lapse_normals <- function(pts_dt, period_label) {
  # Reference period delta: 1991_2020 is ~+0.85 C warmer and +4% winter precip relative to 1961_1990 in BC
  delta_t <- if (identical(period_label, "1991_2020")) 0.85 else 0.0
  precip_mult <- if (identical(period_label, "1991_2020")) 1.03 else 1.00

  zone_means <- bgc_norm_dt[, lapply(.SD, mean, na.rm = TRUE),
    by = Zone,
    .SDcols = c("Latitude", "Longitude", "Elevation", "MAT", "MWMT", "MCMT", "MAP", "MSP",
                "PAS", "CMD", "PPT_05", "PPT_06", "PPT_07", "PPT_08", "PPT_09", "PPT_at", "PPT_wt", "DD5", "NFFD")
  ]

  res_list <- vector("list", nrow(pts_dt))
  for (i in seq_len(nrow(pts_dt))) {
    row <- pts_dt[i]
    bgc_clean <- gsub("[[:space:]]+", "", ifelse(is.na(row$bgc), "", as.character(row$bgc)))
    zone_clean <- gsub("[[:space:]]+", "", ifelse(is.na(row$zone), "", as.character(row$zone)))

    ref <- NULL
    if (nzchar(bgc_clean)) {
      m <- bgc_norm_dt[toupper(BGC) == toupper(bgc_clean)]
      if (nrow(m) > 0L) ref <- m[1L]
    }
    if (is.null(ref) && nzchar(zone_clean)) {
      m <- zone_means[toupper(Zone) == toupper(zone_clean)]
      if (nrow(m) > 0L) ref <- m[1L]
    }
    if (is.null(ref)) {
      # Nearest BGC climate centroid in (lat, lon, elev_km) space
      d2 <- (bgc_norm_dt$Latitude - row$lat)^2 +
            ((bgc_norm_dt$Longitude - row$lon) * cos(row$lat * pi / 180))^2 +
            ((bgc_norm_dt$Elevation - row$elev) / 250.0)^2
      idx <- which.min(d2)
      if (length(idx) == 1L && is.finite(idx)) {
        ref <- bgc_norm_dt[idx]
      } else {
        ref <- bgc_norm_dt[1L]
      }
    }

    # Environmental lapse adjustments relative to reference BGC elevation & latitude
    d_elev_km <- (row$elev - ref$Elevation) / 1000.0
    d_lat <- row$lat - ref$Latitude
    temp_shift <- (-5.8 * d_elev_km) + (-0.55 * d_lat) + delta_t
    elev_precip_factor <- pmax(0.55, pmin(1.65, 1.0 + 0.18 * d_elev_km)) * precip_mult

    mat <- round(ref$MAT + temp_shift, 2)
    mwmt <- round(ref$MWMT + (0.85 * temp_shift), 2)
    mcmt <- round(ref$MCMT + (1.10 * temp_shift), 2)

    ppt05 <- round(pmax(5.0, ref$PPT_05 * elev_precip_factor), 1)
    ppt06 <- round(pmax(5.0, ref$PPT_06 * elev_precip_factor), 1)
    ppt07 <- round(pmax(5.0, ref$PPT_07 * elev_precip_factor), 1)
    ppt08 <- round(pmax(5.0, ref$PPT_08 * elev_precip_factor), 1)
    ppt09 <- round(pmax(5.0, ref$PPT_09 * elev_precip_factor), 1)
    ppt_at <- round(pmax(10.0, ref$PPT_at * elev_precip_factor), 1)
    ppt_wt <- round(pmax(10.0, ref$PPT_wt * elev_precip_factor), 1)
    msp <- round(ppt05 + ppt06 + ppt07 + ppt08 + ppt09, 1)
    map_val <- round(pmax(msp + ppt_at + ppt_wt, ref$MAP * elev_precip_factor), 1)

    dd5 <- round(pmax(25.0, ref$DD5 + (temp_shift * 145.0)), 1)
    nffd <- round(pmax(10.0, pmin(365.0, ref$NFFD + (temp_shift * 14.0))), 1)
    snow_frac <- pmax(0.02, pmin(0.85, 1.0 / (1.0 + exp((mat - 1.5) / 2.2))))
    pas <- round(pmax(0.0, (ppt_wt + 0.45 * ppt_at) * snow_frac), 1)

    # Hargreaves-calibrated CMD shift around reference BGC CMD + Ministry CMD_adjustments.csv
    bgc_adj <- 0.0
    if (nzchar(bgc_clean)) {
      adj_row <- cmd_adj_dt[toupper(ID2) == toupper(bgc_clean)]
      if (nrow(adj_row) > 0L) bgc_adj <- as.numeric(adj_row$CMD_Adjust[[1L]])
    }
    cmd <- round(pmax(0.0, ref$CMD + bgc_adj + (temp_shift * 32.0) - ((msp - ref$MSP) * 0.65)), 1)

    res_list[[i]] <- list(
      coordElevHash = as.character(row$coordElevHash),
      source = "bgc_lapse_model",
      mat = mat,
      mwmt = mwmt,
      mcmt = mcmt,
      map = map_val,
      msp = msp,
      pas = pas,
      cmd = cmd,
      ppt05 = ppt05,
      ppt06 = ppt06,
      ppt07 = ppt07,
      ppt08 = ppt08,
      ppt09 = ppt09,
      pptAt = ppt_at,
      pptWt = ppt_wt,
      dd5 = dd5,
      nffd = nffd
    )
  }
  res_list
}

computed_normals <- list()
if (!is.null(req$uncachedPoints) && length(req$uncachedPoints) > 0L) {
  pts_dt <- data.table::as.data.table(req$uncachedPoints)
  used_climr <- FALSE
  if (can_use_climr_local) {
    climr_try <- tryCatch({
      xyz <- data.table::data.table(
        id = seq_len(nrow(pts_dt)),
        lon = as.numeric(pts_dt$lon),
        lat = as.numeric(pts_dt$lat),
        elev = as.numeric(pts_dt$elev)
      )
      vars_req <- c("MAT", "MWMT", "MCMT", "MAP", "MSP", "PAS", "CMD",
                    "PPT_05", "PPT_06", "PPT_07", "PPT_08", "PPT_09",
                    "PPT_at", "PPT_wt", "DD5", "NFFD")
      dl <- climr::downscale(
        xyz = xyz,
        vars = vars_req,
        return_refperiod = TRUE,
        db_option = "local",
        cache = TRUE
      )
      if (!is.null(dl) && nrow(dl) >= nrow(pts_dt)) {
        dl_dt <- data.table::as.data.table(dl)
        dl_dt <- dl_dt[!duplicated(id)][order(id)]
        pick_col <- function(dt, candidates) {
          for (nm in candidates) {
            if (nm %in% names(dt)) return(as.numeric(dt[[nm]]))
          }
          rep(NA_real_, nrow(dt))
        }
        mat_v <- pick_col(dl_dt, c("MAT"))
        if (all(is.finite(mat_v))) {
          used_climr <<- TRUE
          out_c <- vector("list", nrow(pts_dt))
          ppt05_v <- pick_col(dl_dt, c("PPT_05", "PPT05"))
          ppt06_v <- pick_col(dl_dt, c("PPT_06", "PPT06"))
          ppt07_v <- pick_col(dl_dt, c("PPT_07", "PPT07"))
          ppt08_v <- pick_col(dl_dt, c("PPT_08", "PPT08"))
          ppt09_v <- pick_col(dl_dt, c("PPT_09", "PPT09"))
          for (i in seq_len(nrow(pts_dt))) {
            out_c[[i]] <- list(
              coordElevHash = as.character(pts_dt$coordElevHash[[i]]),
              source = "climr_downscale",
              mat = round(mat_v[[i]], 2),
              mwmt = round(pick_col(dl_dt, c("MWMT"))[[i]], 2),
              mcmt = round(pick_col(dl_dt, c("MCMT"))[[i]], 2),
              map = round(pick_col(dl_dt, c("MAP"))[[i]], 1),
              msp = round(pick_col(dl_dt, c("MSP"))[[i]], 1),
              pas = round(pick_col(dl_dt, c("PAS"))[[i]], 1),
              cmd = round(pick_col(dl_dt, c("CMD", "CMD_an"))[[i]], 1),
              ppt05 = round(ppt05_v[[i]], 1),
              ppt06 = round(ppt06_v[[i]], 1),
              ppt07 = round(ppt07_v[[i]], 1),
              ppt08 = round(ppt08_v[[i]], 1),
              ppt09 = round(ppt09_v[[i]], 1),
              pptAt = round(pick_col(dl_dt, c("PPT_at"))[[i]], 1),
              pptWt = round(pick_col(dl_dt, c("PPT_wt"))[[i]], 1),
              dd5 = round(pick_col(dl_dt, c("DD5"))[[i]], 1),
              nffd = round(pick_col(dl_dt, c("NFFD"))[[i]], 1)
            )
          }
          out_c
        } else {
          NULL
        }
      } else {
        NULL
      }
    }, error = function(e) NULL)

    if (!is.null(climr_try) && used_climr) {
      computed_normals <- climr_try
    }
  }
  if (length(computed_normals) == 0L) {
    computed_normals <- compute_bgc_lapse_normals(pts_dt, ref_period)
  }
}

# Build lookup map from coordElevHash -> climate normals (combining cachedNormals + computed_normals)
norm_env <- new.env(hash = TRUE, parent = emptyenv())
if (!is.null(req$cachedNormals) && length(req$cachedNormals) > 0L) {
  cn_dt <- data.table::as.data.table(req$cachedNormals)
  for (i in seq_len(nrow(cn_dt))) {
    r <- as.list(cn_dt[i])
    assign(as.character(r$coordElevHash), r, envir = norm_env)
  }
}
for (item in computed_normals) {
  assign(as.character(item$coordElevHash), item, envir = norm_env)
}

# Exact Ministry CMD -> aSMR transfer function (_calculate_aSMR.R + aSMR_Rules_HalfStep.csv)
lookup_asmr_rule <- function(rsmr_num, cmd_topo) {
  if (!is.finite(rsmr_num) || !is.finite(cmd_topo)) return(list(cmdSite = NA_real_, asmr = NA_real_))
  r_int <- as.integer(round(rsmr_num))
  if (r_int < 0L || r_int > 8L) return(list(cmdSite = NA_real_, asmr = NA_real_))
  if (r_int == 8L) return(list(cmdSite = 0.0, asmr = 8.0))

  # Site water-balance transformation across shedding (0..3), mesic (4), and receiving (5..7) positions
  cmd_site <- if (r_int < 4L) {
    cmd_topo + (4L - r_int) * 125.0
  } else if (r_int == 4L) {
    cmd_topo
  } else if (r_int == 5L) {
    cmd_topo / 2.0
  } else if (r_int == 6L) {
    cmd_topo / 4.0
  } else {
    cmd_topo / 8.0
  }
  cmd_site <- max(0.0, cmd_site)

  smr_level <- if (r_int <= 4L) 0L else r_int
  sub_rules <- rules_dt[SMRLevel == smr_level & cmd_site <= CMD]
  if (nrow(sub_rules) == 0L) {
    return(list(cmdSite = round(cmd_site, 1), asmr = 0.0))
  }
  best_asmr <- max(as.numeric(sub_rules$aSMR), na.rm = TRUE)
  list(cmdSite = round(cmd_site, 1), asmr = best_asmr)
}

plot_evaluations <- list()
if (!is.null(req$plots) && length(req$plots) > 0L) {
  pl_dt <- data.table::as.data.table(req$plots)
  plot_evaluations <- vector("list", nrow(pl_dt))
  for (i in seq_len(nrow(pl_dt))) {
    pr <- pl_dt[i]
    h <- as.character(pr$coordElevHash)
    clim <- if (nzchar(h) && exists(h, envir = norm_env, inherits = FALSE)) get(h, envir = norm_env) else NULL

    rsmr_clean <- suppressWarnings(as.numeric(sub("^.*?([0-8]).*$", "\\1", ifelse(is.na(pr$rsmr), "", as.character(pr$rsmr)))))
    if (!grepl("[0-8]", ifelse(is.na(pr$rsmr), "", as.character(pr$rsmr)))) {
      rsmr_clean <- NA_real_
    }

    if (is.null(clim)) {
      plot_evaluations[[i]] <- list(
        plotNumber = as.character(pr$plotNumber),
        hasClimate = FALSE,
        rsmrNumeric = if (is.finite(rsmr_clean)) rsmr_clean else NULL,
        cmdTopo = NULL,
        cmdSite = NULL,
        asmr = NULL
      )
      next
    }

    # Growing-season moisture surplus check (_calculate_aSMR.R lines 10-27)
    p59 <- c(clim$ppt05, clim$ppt06, clim$ppt07, clim$ppt08, clim$ppt09)
    moist_cnt <- sum(p59 >= 90.0, na.rm = TRUE)
    moist_flag <- if (moist_cnt > 4L) 1.0 else 0.0
    multiplier <- 1.0 + (moist_flag / 5.0)
    cmd_gs <- as.numeric(clim$cmd) / multiplier

    # Dormant-season recharge deficit (_calculate_aSMR.R lines 29-40)
    ppt_dorm <- as.numeric(clim$pptAt) + as.numeric(clim$pptWt)
    def_val <- max(0.0, 350.0 - ppt_dorm)
    cmd_total <- cmd_gs + def_val

    # Steep warm/cool aspect factor (_calculate_aSMR.R lines 41-54)
    slope_v <- suppressWarnings(as.numeric(pr$slope))
    aspect_v <- suppressWarnings(as.numeric(pr$aspect))
    aspect_factor <- 1.0
    if (is.finite(slope_v) && is.finite(aspect_v) && slope_v > 35.0) {
      if (aspect_v > 180.0 && aspect_v < 270.0) {
        aspect_factor <- 1.25
      } else if (aspect_v > 315.0 || (aspect_v >= 0.0 && aspect_v < 45.0)) {
        aspect_factor <- 0.85
      }
    }
    cmd_topo <- round(cmd_total * aspect_factor, 1)
    mapped <- lookup_asmr_rule(rsmr_clean, cmd_topo)

    plot_evaluations[[i]] <- list(
      plotNumber = as.character(pr$plotNumber),
      hasClimate = TRUE,
      rsmrNumeric = if (is.finite(rsmr_clean)) rsmr_clean else NULL,
      moistMonths = as.integer(moist_cnt),
      pptDormant = round(ppt_dorm, 1),
      dormantDeficit = round(def_val, 1),
      aspectFactor = aspect_factor,
      cmdTopo = cmd_topo,
      cmdSite = if (is.finite(mapped$cmdSite)) mapped$cmdSite else NULL,
      asmr = if (is.finite(mapped$asmr)) mapped$asmr else NULL
    )
  }
}

resp <- list(
  ok = TRUE,
  refPeriod = ref_period,
  vproCompatible = vpro_compatible,
  climrUsed = can_use_climr_local && length(computed_normals) > 0L && identical(computed_normals[[1L]]$source, "climr_downscale"),
  computedNormals = computed_normals,
  plotEvaluations = plot_evaluations
)

jsonlite::write_json(resp, path = resp_path, auto_unbox = TRUE, pretty = FALSE, null = "null")
