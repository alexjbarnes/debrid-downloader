# Grouped Downloads Feature

## Overview

The grouped downloads feature allows multi-file downloads to be displayed as single expandable cards instead of individual items, providing a cleaner UI and better organization. Single-file downloads continue to display as individual cards.

## Architecture

### Core Components

#### 1. Data Models (`pkg/models/download.go`)

**Download Model Extensions:**
- `IsExpanded bool`: Tracks UI expand/collapse state for individual downloads

**DownloadDisplayItem Model:**
```go
type DownloadDisplayItem struct {
    IsGroup               bool                      // true for grouped, false for single
    Download              *Download                 // For single downloads
    GroupID               string                    // For grouped downloads
    Downloads             []*Download               // All downloads in the group
    HighestPriorityStatus DownloadStatus           // Most important status in group
    StatusCounts          map[DownloadStatus]int   // Count of each status
    OverallProgress       float64                  // Average progress across group
    HasActiveDownloads    bool                     // Whether group has pending/downloading items
    IsExpanded            bool                     // UI expand/collapse state for group
}
```

#### 2. Server-Side State Management (`internal/web/handlers/handlers.go`)

**State Tracking:**
- `expandedDownloads map[int64]bool`: Per-download expand state
- `expandedGroups map[string]bool`: Per-group expand state
- `expandMutex sync.RWMutex`: Thread-safe access to state

**Default Expand Behavior:**
- Single downloads: Expanded if downloading, collapsed otherwise
- Groups: Expanded if any download is active (pending/downloading/paused)
- User interactions override defaults and persist until page reload

#### 3. Display Item Creation Logic

The `createDisplayItems()` function:
1. **Maintains Sort Order**: Processes downloads in original time-based order
2. **Groups Related Downloads**: Collects downloads by `GroupID`
3. **Creates Mixed List**: Single downloads and group items interleaved by time
4. **Calculates Group Statistics**: Status counts, overall progress, priority status

**Grouping Logic:**
```go
// Single-file download (GroupID == "")
displayItems = append(displayItems, &DownloadDisplayItem{
    IsGroup:  false,
    Download: download,
})

// Multi-file download (first occurrence of GroupID)
displayItems = append(displayItems, &DownloadDisplayItem{
    IsGroup:               true,
    GroupID:               download.GroupID,
    Downloads:             groupDownloads,
    HighestPriorityStatus: calculatePriorityStatus(groupDownloads),
    StatusCounts:          countStatuses(groupDownloads),
    OverallProgress:       calculateAverageProgress(groupDownloads),
    HasActiveDownloads:    hasActiveDownloads(groupDownloads),
    IsExpanded:            isGroupExpanded(download.GroupID, hasActive),
})
```

#### 4. Hybrid Polling System

**Two-Tier Polling Strategy:**
- **Fast Progress Updates (500ms)**: Updates only progress bars and download speeds
- **Slow Full Refresh (30s)**: Full list refresh to catch new downloads

**Implementation:**
- Static polling triggers in DOM prevent HTMX initialization issues
- Progress endpoint returns individual item updates via `hx-swap-oob`
- Full refresh replaces entire downloads list

## UI Components

### 1. Single Download Display (`DownloadItem`)
- Standard download card with expand/collapse functionality
- Click header to toggle detailed view
- Server-side state management via HTMX

### 2. Grouped Download Display (`DownloadGroupItem`)
- Single card representing entire group
- Status badge counts for quick overview
- Overall progress bar when group has active downloads
- Expandable to show individual download items

### 3. Sub-Item Display (`DownloadSubItem`)
- Individual downloads within a group
- Compact header with filename and status
- Progress bar always visible for downloading items
- Expandable for full details (directory, size, actions)

## API Endpoints

### Toggle Endpoints
- `POST /downloads/{id}/toggle`: Toggle individual download expand state
- `POST /groups/{id}/toggle`: Toggle group expand state

### Progress Updates
- `POST /downloads/progress`: Fast progress updates for active downloads only
- Returns out-of-band HTML updates for changed items

## User Experience

### Interaction Flow
1. **Page Load**: Downloads grouped automatically, active items expanded
2. **User Interaction**: Click any header to expand/collapse
3. **State Persistence**: Expand states maintained until page refresh
4. **Real-time Updates**: Progress bars update every 500ms
5. **New Downloads**: Full refresh every 30s catches new items

### Visual Hierarchy
```
Single Download
├── Header (clickable)
│   ├── Status Badge
│   ├── Filename
│   └── Chevron Icon
└── Details (collapsible)
    ├── URL, Directory, Size
    ├── Progress Bar (if downloading)
    └── Action Buttons

Group Download
├── Group Header (clickable)
│   ├── Status Badge Counts
│   ├── Group Title
│   └── Chevron Icon
├── Group Progress Bar (if active)
└── Individual Downloads (collapsible)
    ├── Sub-Item 1
    │   ├── Compact Header (clickable)
    │   ├── Progress Bar (if downloading)
    │   └── Full Details (collapsible)
    └── Sub-Item 2...
```

## Technical Considerations

### Performance
- Progress updates only affect active downloads (downloading status)
- OOB updates minimize DOM manipulation
- Expand state cached in memory for fast access

### Scalability
- Groups can contain unlimited downloads
- Progress calculations scale linearly with group size
- State maps use efficient lookups by ID

### Error Handling
- Missing downloads gracefully handled in toggle endpoints
- Invalid group IDs return 404 responses
- Database errors logged and return 500 responses

### Browser Compatibility
- Uses HTMX for progressive enhancement
- Falls back gracefully without JavaScript
- Responsive design works on all screen sizes

## Testing Strategy

### Unit Tests
- `createDisplayItems()` function with various input scenarios
- Expand state management functions
- Toggle handler logic
- Progress update filtering

### Integration Tests
- Full grouped download workflow
- HTMX interaction simulation
- Polling behavior verification
- State persistence across requests

### Edge Cases
- Empty groups
- Single-item groups
- Mixed group and single downloads
- Rapid state changes
- Network interruptions

## Future Enhancements

### Potential Improvements
- Persistent expand state across sessions (database storage)
- Group-level actions (pause all, retry all)
- Custom grouping rules beyond GroupID
- Bulk operations on selected downloads
- Real-time WebSocket updates instead of polling

### Monitoring
- Track expand/collapse usage patterns
- Monitor polling performance impact
- Measure user engagement with grouped view