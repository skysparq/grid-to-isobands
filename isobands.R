library(sf)
library(dplyr)
library(s2)

only_polys <- function(geometry) {
  geometry %>%
    filter(st_geometry_type(.) %in% c("POLYGON", "MULTIPOLYGON"))
}

flip_large_polygons <- function(data, area_threshold = 2.5e14) {
  for (i in 1:nrow(data)) {
    geom <- st_geometry(data[i,])
    area <- as.numeric(st_area(geom))
    
    if (area > area_threshold) {
      # Use s2 to rebuild with correct orientation
      st_geometry(data[i,]) <- geom %>%
        s2::as_s2_geography() %>%
        s2::s2_rebuild(s2_options(dimensions = "polygon")) %>%
        st_as_sfc()
    }
  }
  return(data)
}

process_isobands <- function(input_path, output_path, tolerance_meters = 100) {
  cat("Reading GeoJSON...\n")
  data <- st_read(input_path, quiet = TRUE)
  
  data <- flip_large_polygons(data)

  cat(sprintf("Read %d features\n", nrow(data)))

  #cross_edge <- 
  #  st_is_valid(data, reason = TRUE) |>
  #  grepl(x = _, pattern = "edge .* crosses edge", ignore.case = TRUE)
  
  #st_geometry(data)[cross_edge] <-
  #  st_geometry(data)[cross_edge] |>  
  #  as_s2_geography(check = FALSE) |>
  #  s2_union() |>
  #  st_as_sfc() 
  
  simplified <- st_simplify(data, preserveTopology = TRUE, dTolerance = tolerance_meters)
  simplified <- only_polys(simplified)
  
  # Group by level
  levels <- unique(simplified$levelIndex)
  levels <- sort(levels, decreasing = TRUE)  # Highest to lowest
  
  levels_without_holes <- list()
  for (level_idx in levels) {
    level_polygons <- simplified %>% filter(levelIndex == level_idx)
    levels_without_holes[[as.character(level_idx)]] <- level_polygons
  }
  
  # Punch holes between levels (higher levels punch into lower)
  cat("Punching holes between levels...\n")
  final_results <- list()
  accumulated_mask <- NULL
  
  i <- 0
  for (level_idx in levels) {
    level_key <- as.character(level_idx)
    current_level <- levels_without_holes[[level_key]]
    cat(sprintf("Punching level %d...\n", i))
    i <- i + 1

    # Erase accumulated mask from current level
    if (!is.null(accumulated_mask)) {
      tryCatch({
        current_level <- st_make_valid(st_difference(current_level, accumulated_mask))
        current_level <- only_polys(current_level)
      }, error = function(e) {
        cat(sprintf("  Warning: Error punching level %d: %s\n", level_idx, e$message))
      })
    }

    # Remove empty geometries
    if (nrow(current_level) > 0) {
      current_level <- current_level[!st_is_empty(current_level), ]
    }

    if (nrow(current_level) > 0) {
      final_results[[level_key]] <- lapply(1:nrow(current_level), function(i) {
          st_cast(current_level[i, ], "POLYGON")
        }) %>% do.call(rbind, .)
    }

    # Add to accumulated mask
    if (is.null(accumulated_mask)) {
      accumulated_mask <- st_make_valid(st_union(current_level))
    } else {
      current_union <- st_make_valid(st_union(current_level))
      accumulated_mask <- st_make_valid(st_union(accumulated_mask, current_union))
    }
  }

  # Combine results
  if (length(final_results) > 0) {
    all_features <- do.call(rbind, final_results)

    cat(sprintf("Complete: %d features\n", nrow(all_features)))

    st_write(all_features, output_path, delete_dsn = TRUE, quiet = TRUE)

    cat(sprintf("Wrote output to %s\n", output_path))
  } else {
    cat("No valid features to write\n")
  }
}

# Usage
args <- commandArgs(trailingOnly = TRUE)
if (length(args) < 2) {
  stop("Usage: Rscript isobands.R input.json output.json [tolerance_meters]")
}

input_path <- args[1]
output_path <- args[2]
tolerance_meters <- if (length(args) >= 3) as.numeric(args[3]) else 100

process_isobands(input_path, output_path, tolerance_meters)