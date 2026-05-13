import * as React from "react"
import { DayPicker } from "react-day-picker"

import { cn } from "@/lib/utils"
import { buttonVariants } from "@/components/ui/button"

function Calendar({
  className,
  classNames,
  showOutsideDays = true,
  ...props
}: React.ComponentProps<typeof DayPicker>) {
  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      className={cn("p-3", className)}
      classNames={{
        button_next: cn(buttonVariants({ variant: "ghost", size: "icon-sm" }), "size-8 rounded-md"),
        button_previous: cn(buttonVariants({ variant: "ghost", size: "icon-sm" }), "size-8 rounded-md"),
        caption_label: "text-sm font-medium text-foreground",
        day: "text-center text-sm p-0 relative [&:has([aria-selected])]:bg-accent first:[&:has([aria-selected])]:rounded-l-md last:[&:has([aria-selected])]:rounded-r-md focus-within:relative focus-within:z-20",
        day_button: cn(buttonVariants({ variant: "ghost", size: "icon-sm" }), "size-9 rounded-md p-0 font-normal aria-selected:opacity-100"),
        disabled: "text-muted-foreground opacity-50",
        hidden: "invisible",
        month: "space-y-4",
        month_grid: "w-full border-collapse space-y-1",
        months: "flex flex-col sm:flex-row gap-4",
        nav: "flex items-center gap-1",
        outside: "text-muted-foreground opacity-50 aria-selected:bg-accent/50 aria-selected:text-muted-foreground aria-selected:opacity-30",
        root: "w-fit",
        selected: "bg-primary text-primary-foreground hover:bg-primary hover:text-primary-foreground focus:bg-primary focus:text-primary-foreground",
        today: "bg-accent text-accent-foreground",
        week: "flex w-full mt-2",
        weekday: "text-muted-foreground rounded-md w-9 font-normal text-[0.8rem]",
        weekdays: "flex",
        ...classNames,
      }}
      {...props}
    />
  )
}

export { Calendar }
