import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";

import { cn } from "@/lib/utils";

/**
 * PILOT COMMAND Button Component
 * Industrial mechanical design with sharp corners and solid offset shadows.
 * Features mechanical press effect on :active state.
 */
const buttonVariants = cva(
	[
		// Base mechanical style
		"inline-flex items-center justify-center gap-2 whitespace-nowrap",
		"text-sm font-semibold uppercase tracking-wide",
		"rounded-md", // Sharp corners - max rounded-md
		"border border-transparent",
		"transition-[transform,box-shadow] duration-50 ease-out",
		// Press effect
		"active:translate-y-0.5 active:shadow-none",
		// Focus and disabled states
		"outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
		"disabled:pointer-events-none disabled:opacity-50",
		// SVG handling
		"[&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0",
	].join(" "),
	{
		variants: {
			variant: {
				// Primary: Amber/orange accent - industrial warning/action color
				default: [
					"bg-[oklch(0.75_0.18_55)] text-[oklch(0.15_0.02_55)]",
					"shadow-[0_2px_0_0_oklch(0_0_0/0.25)]",
					"hover:bg-[oklch(0.78_0.19_55)] hover:border-[oklch(0.65_0.2_55)]",
					"focus-visible:ring-[oklch(0.75_0.18_55)]",
				].join(" "),

				// Secondary: Dark panel background with visible border
				secondary: [
					"bg-[oklch(0.18_0.04_260)] text-[oklch(0.85_0.02_260)]",
					"border-[oklch(0.30_0.05_260)]",
					"shadow-[0_2px_0_0_oklch(0_0_0/0.3)]",
					"hover:bg-[oklch(0.22_0.05_260)] hover:border-[oklch(0.75_0.18_55)]",
					"focus-visible:ring-[oklch(0.45_0.08_260)]",
				].join(" "),

				// Destructive: Muted red-brown for danger actions
				destructive: [
					"bg-[oklch(0.35_0.12_25)] text-[oklch(0.90_0.02_25)]",
					"border-[oklch(0.40_0.15_25)]",
					"shadow-[0_2px_0_0_oklch(0_0_0/0.3)]",
					"hover:bg-[oklch(0.40_0.14_25)] hover:border-[oklch(0.50_0.18_25)]",
					"focus-visible:ring-[oklch(0.45_0.15_25)]",
				].join(" "),

				// Outline: Border only, amber border on hover
				outline: [
					"bg-transparent text-foreground",
					"border-[oklch(0.35_0.05_260)]",
					"shadow-[0_2px_0_0_oklch(0_0_0/0.1)]",
					"hover:border-[oklch(0.75_0.18_55)] hover:text-[oklch(0.75_0.18_55)]",
					"focus-visible:ring-[oklch(0.75_0.18_55)]",
				].join(" "),

				// Ghost: Transparent with subtle hover
				ghost: [
					"bg-transparent text-foreground",
					"shadow-none",
					"hover:bg-[oklch(0.25_0.04_260)] hover:text-[oklch(0.85_0.02_260)]",
					"active:translate-y-0 active:bg-[oklch(0.20_0.04_260)]",
					"focus-visible:ring-[oklch(0.45_0.08_260)]",
				].join(" "),

				// Command: Industrial command button with glow effect
				command: [
					"bg-[oklch(0.20_0.05_260)] text-[oklch(0.75_0.18_55)]",
					"border-[oklch(0.75_0.18_55)]",
					"shadow-[0_2px_0_0_oklch(0.75_0.18_55/0.4),0_0_12px_0_oklch(0.75_0.18_55/0.2)]",
					"hover:bg-[oklch(0.25_0.06_260)] hover:shadow-[0_2px_0_0_oklch(0.75_0.18_55/0.6),0_0_20px_0_oklch(0.75_0.18_55/0.35)]",
					"hover:text-[oklch(0.80_0.20_55)]",
					"active:shadow-[0_0_8px_0_oklch(0.75_0.18_55/0.4)]",
					"focus-visible:ring-[oklch(0.75_0.18_55)]",
				].join(" "),

				// Link: Text-only navigation style
				link: [
					"text-[oklch(0.75_0.18_55)] underline-offset-4",
					"shadow-none",
					"hover:underline hover:text-[oklch(0.80_0.20_55)]",
					"active:translate-y-0",
					"focus-visible:ring-[oklch(0.75_0.18_55)]",
				].join(" "),
			},
			size: {
				default: "h-9 px-4 py-2 has-[>svg]:px-3",
				sm: "h-8 rounded-md gap-1.5 px-3 text-xs has-[>svg]:px-2.5",
				lg: "h-11 rounded-md px-6 text-base has-[>svg]:px-4",
				icon: "size-9",
				"icon-sm": "size-8",
				"icon-lg": "size-10",
			},
		},
		defaultVariants: {
			variant: "default",
			size: "default",
		},
	},
);

function Button({
	className,
	variant = "default",
	size = "default",
	asChild = false,
	...props
}: React.ComponentProps<"button"> &
	VariantProps<typeof buttonVariants> & {
		asChild?: boolean;
	}) {
	const Comp = asChild ? Slot : "button";

	return (
		<Comp
			data-slot="button"
			data-variant={variant}
			data-size={size}
			className={cn(buttonVariants({ variant, size, className }))}
			{...props}
		/>
	);
}

export { Button, buttonVariants };
