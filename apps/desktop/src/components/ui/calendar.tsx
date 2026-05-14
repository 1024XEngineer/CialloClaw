import * as React from "react"
import { format } from "date-fns"
import { zhCN } from "date-fns/locale"
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
      locale={zhCN}
      showOutsideDays={showOutsideDays}
      weekStartsOn={1}
      className={cn("p-0", className)}
      classNames={{
        button_next: cn(buttonVariants({ variant: "ghost", size: "icon-sm" }), "size-7 rounded-full text-[#7a5a33] hover:bg-white/70"),
        button_previous: cn(buttonVariants({ variant: "ghost", size: "icon-sm" }), "size-7 rounded-full text-[#7a5a33] hover:bg-white/70"),
        caption_label: "text-sm font-medium text-[#5b3f24]",
        day: "relative text-center text-sm p-0 focus-within:relative focus-within:z-20",
        day_button: cn(
          buttonVariants({ variant: "ghost", size: "icon-sm" }),
          "size-8 rounded-full border border-transparent p-0 font-normal text-[#6f5535] hover:bg-[rgba(232,190,104,0.2)] hover:text-[#5b3f24] aria-selected:rounded-full aria-selected:border-[rgba(210,160,70,0.35)] aria-selected:bg-[rgba(232,190,104,0.35)] aria-selected:text-[#5b3f24]",
        ),
        disabled: "text-muted-foreground opacity-35",
        hidden: "invisible",
        month: "space-y-3",
        month_grid: "w-full border-collapse",
        months: "flex flex-col gap-3",
        nav: "flex items-center gap-1",
        outside: "text-[#b5a389] opacity-70",
        root: "w-fit",
        selected: "",
        today: "text-[#5b3f24]",
        week: "flex w-full mt-1.5",
        weekday: "w-8 text-center text-[0.75rem] font-medium text-[#8d7350]",
        weekdays: "flex mb-1",
        ...classNames,
      }}
      formatters={{
        formatCaption: (month) => format(month, "yyyy年M月", { locale: zhCN }),
        formatWeekdayName: (date) => format(date, "EEEEE", { locale: zhCN }),
      }}
      {...props}
    />
  )
}

export { Calendar }
