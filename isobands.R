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
  data <- st_set_precision(data, 10000)
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
      
      combined_geom <- fills %>%
        rowwise() %>%
        mutate(
          geometry = {
            fill_geom <- geometry  # Capture the current fill's geometry
            
            if (nrow(holes) > 0) {
              # Find holes that intersect this fill
              intersects <- st_intersects(fill_geom, st_sfc(holes$geometry, crs = st_crs(fill_geom)), sparse = FALSE)[1,]
              relevant_holes <- holes[intersects, ]
              
              if (nrow(relevant_holes) > 0) {
                # Check which ones completely contain the fill
                covered <- st_covered_by(fill_geom, st_sfc(relevant_holes$geometry, crs = st_crs(fill_geom)), sparse = FALSE)[1,]
                relevant_holes <- relevant_holes[!covered, ]
              }
              
              if (nrow(relevant_holes) > 0) {
                combined_holes <- st_union(st_sfc(relevant_holes$geometry, crs = st_crs(fill_geom)))
                
                # Try difference with error handling
                diff_result <- tryCatch({
                  result <- st_difference(fill_geom, combined_holes, dimension='polygon')
                  
                  # Check if result is valid
                  if (length(result) == 0 || all(st_is_empty(result))) {
                    fill_geom
                  } else {
                    result
                  }
                }, error = function(e) {
                  # If difference fails due to invalid geometry, try to make valid first
                  tryCatch({
                    valid_fill <- st_make_valid(fill_geom)
                    valid_holes <- st_make_valid(combined_holes)
                    result <- st_difference(valid_fill, valid_holes, dimension='polygon')
                    
                    if (length(result) == 0 || all(st_is_empty(result))) {
                      fill_geom
                    } else {
                      result
                    }
                  }, error = function(e2) {
                    # If still fails, return original geometry
                    fill_geom
                  })
                })
                
                diff_result
              } else {
                fill_geom
              }
            } else {
              fill_geom
            }
          }
        ) %>%
        ungroup()

      # Handle any remaining invalid geometries before converting to sf
      combined_geom <- combined_geom %>%
        filter(!st_is_empty(geometry)) %>%
        st_as_sf() %>%
        st_make_valid()
        
      # Convert to sf if not empty
      if (length(combined_geom) > 0 && !all(st_is_empty(combined_geom))) {
        combined_sf <- st_make_valid(combined_geom)
        combined_sf <- only_polys(combined_sf)

        levels_combined[[as.character(level_idx)]] <- combined_sf
      }
    }
    
    # Punch holes between levels (higher levels punch into lower)
    final_results_q <- list()
    accumulated_mask <- NULL
    
    for (level_idx in levels) {
      level_key <- as.character(level_idx)
      if (!level_key %in% names(levels_combined)) next
      
      current_level <- levels_combined[[level_key]]
      punched <- current_level

      # Erase accumulated mask from current level
      if (!is.null(accumulated_mask)) {
        punched <- st_difference(current_level, accumulated_mask, dimension='polygon')
        punched <- st_make_valid(punched)
        punched <- only_polys(punched)
      }
      
      # Remove empty geometries
      if (nrow(punched) > 0) {
        punched <- punched[!st_is_empty(punched), ]
      }
      
      if (nrow(punched) > 0) {
        casted <- lapply(1:nrow(punched), function(i) {
          st_cast(punched[i, ], "POLYGON")
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
  all_features <- do.call(rbind, final_results)
  st_write(all_features, output_path, delete_dsn = TRUE, quiet = TRUE)
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