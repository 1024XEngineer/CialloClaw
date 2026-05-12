import type { ButtonHTMLAttributes } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center rounded-2xl border text-sm font-medium transition-all duration-300 disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        primary:
          "border border-white/12 bg-[linear-gradient(180deg,rgba(113,146,214,0.48),rgba(66,86,126,0.42))] px-6 py-3 text-white shadow-[0_18px_44px_rgba(53,87,153,0.22)] backdrop-blur-md hover:-translate-y-1 hover:bg-[linear-gradient(180deg,rgba(132,169,245,0.58),rgba(79,104,156,0.5))] hover:shadow-[0_22px_56px_rgba(83,125,214,0.28)]",
        secondary:
          "border border-white/10 bg-[linear-gradient(180deg,rgba(95,116,160,0.24),rgba(52,66,103,0.18))] px-6 py-3 text-white/92 shadow-[0_16px_40px_rgba(28,35,59,0.18)] backdrop-blur-md hover:-translate-y-1 hover:bg-[linear-gradient(180deg,rgba(121,148,203,0.3),rgba(70,87,131,0.24))] hover:shadow-[0_20px_48px_rgba(54,75,126,0.22)]",
        ghost:
          "border-transparent px-4 py-2.5 text-[color:var(--cc-ink-soft)] hover:bg-[color:var(--cc-surface)] hover:text-[color:var(--cc-ink)]",
      },
    },
    defaultVariants: {
      variant: "primary",
    },
  },
);

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof buttonVariants>;

export function Button({ className, variant, ...props }: ButtonProps) {
  return <button className={cn(buttonVariants({ variant }), className)} {...props} />;
}
