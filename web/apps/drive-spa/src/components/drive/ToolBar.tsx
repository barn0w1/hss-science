import { LayoutGrid, List, ChevronDown, FolderPlus } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useUIStore } from "@/store/ui.store"
import type { SortField, SortDir } from "@/store/ui.store"
import { cn } from "@/lib/utils"

const SORT_OPTIONS: { label: string; field: SortField; dir: SortDir }[] = [
  { label: "Name (A → Z)", field: "name", dir: "asc" },
  { label: "Name (Z → A)", field: "name", dir: "desc" },
  { label: "Modified (newest)", field: "updatedAt", dir: "desc" },
  { label: "Modified (oldest)", field: "updatedAt", dir: "asc" },
  { label: "Size (largest)", field: "size", dir: "desc" },
  { label: "Size (smallest)", field: "size", dir: "asc" },
]

export function ToolBar() {
  const viewMode = useUIStore((s) => s.viewMode)
  const sortField = useUIStore((s) => s.sortField)
  const sortDir = useUIStore((s) => s.sortDir)
  const setViewMode = useUIStore((s) => s.setViewMode)
  const setSort = useUIStore((s) => s.setSort)
  const openNewFolderDialog = useUIStore((s) => s.openNewFolderDialog)

  const activeSortLabel =
    SORT_OPTIONS.find((o) => o.field === sortField && o.dir === sortDir)
      ?.label ?? "Sort"

  return (
    <div className="flex shrink-0 items-center gap-2 border-b border-border px-4 py-2">
      <Button
        size="sm"
        variant="outline"
        className="gap-1.5"
        onClick={openNewFolderDialog}
      >
        <FolderPlus className="h-3.5 w-3.5" />
        New folder
      </Button>

      <div className="ml-auto flex items-center gap-1">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="sm" className="gap-1.5 text-xs">
              {activeSortLabel}
              <ChevronDown className="h-3.5 w-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            {SORT_OPTIONS.map((opt, i) => (
              <span key={`${opt.field}-${opt.dir}`}>
                {i === 2 && <DropdownMenuSeparator />}
                {i === 4 && <DropdownMenuSeparator />}
                <DropdownMenuItem
                  onClick={() => setSort(opt.field, opt.dir)}
                  className={cn(
                    sortField === opt.field &&
                      sortDir === opt.dir &&
                      "font-medium text-primary"
                  )}
                >
                  {opt.label}
                </DropdownMenuItem>
              </span>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        <div className="flex rounded-md border border-border">
          <Button
            variant="ghost"
            size="icon-sm"
            className={cn(
              "rounded-r-none border-0",
              viewMode === "grid" && "bg-accent"
            )}
            onClick={() => setViewMode("grid")}
          >
            <LayoutGrid className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            className={cn(
              "rounded-l-none border-0 border-l border-border",
              viewMode === "list" && "bg-accent"
            )}
            onClick={() => setViewMode("list")}
          >
            <List className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  )
}
