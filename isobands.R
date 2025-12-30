library(sf)
library(dplyr)
library(s2)

only_polys <- function(geometry) {
  geometry %>%
    filter(st_geometry_type(.) %in% c("POLYGON", "MULTIPOLYGON"))
}

process_isobands <- function(input_path, output_path, tolerance_meters = 100) {
  options(s2_oriented = TRUE)
  sf_use_s2(TRUE)
  data <- st_read(input_path, quiet = TRUE)
  data <- st_make_valid(data)

  simplified <- st_simplify(data, preserveTopology = TRUE, dTolerance = tolerance_meters)
  simplified <- only_polys(simplified)
  
  # Group by quadrant
  quadrants <- unique(simplified$quadrant)
  quadrants <- sort(quadrants)

  final_results <- list()
  
  for (q in quadrants) {
    quad_data <- simplified %>% filter(quadrant == q)
    
    # Group by level
    levels <- unique(quad_data$levelIndex)
    levels <- sort(levels, decreasing = TRUE)  # Highest to lowest
    
    levels_combined <- list()
    for (level_idx in levels) {
      level_polys <- quad_data %>% filter(levelIndex == level_idx)
      if (nrow(level_polys) == 0) next
      
      level_polys <- st_make_valid(level_polys)
      level_polys$area <- as.numeric(st_area(level_polys))
      
      fills <- level_polys %>% filter(!isHole) %>% arrange(desc(area))
      holes <- level_polys %>% filter(isHole) %>% arrange(desc(area))
      
      # Initialize with union of fills
      if (nrow(fills) > 0) {
        combined_geom <- st_union(st_sfc(fills$geometry, crs = st_crs(fills)))
        combined_geom <- st_make_valid(combined_geom)
      } else {
        combined_geom <- st_sfc(crs = st_crs(level_polys))
      }
      
      # Then subtract holes one by one
      for (i in 1:nrow(holes)) {
        hole_geom <- holes$geometry[i]
        area <- holes$area[i]

        inter <- st_intersects(combined_geom, hole_geom, sparse = FALSE)
        if (any(inter)) {
          tryCatch({
            combined_geom <- st_difference(combined_geom, hole_geom)
            combined_geom <- st_make_valid(combined_geom)
          }, error = function(e) {
            cat(sprintf("Warning: Error differencing hole: %s\n", e$message))
          })
        }
      }
      
      # Convert to sf if not empty
      if (length(combined_geom) > 0 && !all(st_is_empty(combined_geom))) {
        combined_sf <- st_sf(geometry = combined_geom, levelIndex = level_idx, quadrant = q, floor = level_polys[["floor"]][1], ceiling = level_polys[["ceiling"]][1])
        combined_sf <- st_make_valid(combined_sf)
        combined_sf <- only_polys(combined_sf)

        levels_combined[[as.character(level_idx)]] <- combined_sf
      }
    }
    
    # Punch holes between levels (higher levels punch into lower)
    final_results_q <- list()
    accumulated_mask <- NULL
    
    i <- 0
    for (level_idx in levels) {
      level_key <- as.character(level_idx)
      if (!level_key %in% names(levels_combined)) next
      
      current_level <- levels_combined[[level_key]]
      i <- i + 1
      
      # Erase accumulated mask from current level
      if (!is.null(accumulated_mask)) {
        current_level <- st_difference(current_level, accumulated_mask, dimension='polygon')
        current_level <- st_make_valid(current_level)
        current_level <- only_polys(current_level)
      }
      
      # Remove empty geometries
      if (nrow(current_level) > 0) {
        current_level <- current_level[!st_is_empty(current_level), ]
      }
      
      if (nrow(current_level) > 0) {
        casted <- lapply(1:nrow(current_level), function(i) {
          st_cast(current_level[i, ], "POLYGON")
        }) %>% do.call(rbind, .)
        final_results_q[[level_key]] <- casted
      }
      
      # Add to accumulated mask
      if (nrow(current_level) > 0) {
        current_union <- st_make_valid(st_union(current_level))
        
        if (is.null(accumulated_mask)) {
          accumulated_mask <- current_union
        } else {
          accumulated_mask <- st_make_valid(st_union(accumulated_mask, current_union))
        }
      }
    }
    
    # Append to global final_results
    for (key in names(final_results_q)) {
      if (key %in% names(final_results)) {
        final_results[[key]] <- rbind(final_results[[key]], final_results_q[[key]])
      } else {
        final_results[[key]] <- final_results_q[[key]]
      }
    }
  }
  
  # Combine results
  if (length(final_results) > 0) {
    all_features <- do.call(rbind, final_results)
    st_write(all_features, output_path, delete_dsn = TRUE, quiet = TRUE)
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