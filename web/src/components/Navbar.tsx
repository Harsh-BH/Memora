"use client";

import React, { useLayoutEffect, useRef, useState, useCallback } from "react";
import { gsap } from "gsap";
import { GoArrowUpRight } from "react-icons/go";

type CardNavLink = {
    label: string;
    href: string;
    ariaLabel: string;
};

type CardNavItem = {
    label: string;
    bgColor: string;
    textColor: string;
    links: CardNavLink[];
};

const items: CardNavItem[] = [
    {
        label: "Product",
        bgColor: "#4A4947",
        textColor: "#FAF7F0",
        links: [
            { label: "Features", href: "#features", ariaLabel: "View Features" },
            { label: "How It Works", href: "#steps", ariaLabel: "How It Works" },
        ],
    },
    {
        label: "Resources",
        bgColor: "#4A4947",
        textColor: "#FAF7F0",
        links: [
            { label: "Documentation", href: "#", ariaLabel: "API Documentation" },
            { label: "GitHub", href: "#", ariaLabel: "GitHub Repository" },
            { label: "Blog", href: "#", ariaLabel: "Blog" },
        ],
    },
    {
        label: "Company",
        bgColor: "#4A4947",
        textColor: "#FAF7F0",
        links: [
            { label: "About", href: "#", ariaLabel: "About Memora" },
            { label: "Contact", href: "#", ariaLabel: "Contact Us" },
            { label: "Pricing", href: "#", ariaLabel: "Pricing" },
        ],
    },
];

export default function Navbar() {
    const [isHamburgerOpen, setIsHamburgerOpen] = useState(false);
    const [isExpanded, setIsExpanded] = useState(false);
    const navRef = useRef<HTMLDivElement | null>(null);
    const cardsRef = useRef<HTMLDivElement[]>([]);
    const tlRef = useRef<gsap.core.Timeline | null>(null);

    const calculateHeight = useCallback(() => {
        const navEl = navRef.current;
        if (!navEl) return 260;

        const isMobile = window.matchMedia("(max-width: 768px)").matches;
        if (isMobile) {
            const contentEl = navEl.querySelector(
                ".card-nav-content"
            ) as HTMLElement;
            if (contentEl) {
                const wasVisible = contentEl.style.visibility;
                const wasPointerEvents = contentEl.style.pointerEvents;
                const wasPosition = contentEl.style.position;
                const wasHeight = contentEl.style.height;

                contentEl.style.visibility = "visible";
                contentEl.style.pointerEvents = "auto";
                contentEl.style.position = "static";
                contentEl.style.height = "auto";

                contentEl.offsetHeight;

                const topBar = 60;
                const padding = 16;
                const contentHeight = contentEl.scrollHeight;

                contentEl.style.visibility = wasVisible;
                contentEl.style.pointerEvents = wasPointerEvents;
                contentEl.style.position = wasPosition;
                contentEl.style.height = wasHeight;

                return topBar + contentHeight + padding;
            }
        }
        return 260;
    }, []);

    const createTimeline = useCallback(() => {
        const navEl = navRef.current;
        if (!navEl) return null;

        gsap.set(navEl, { height: 60, overflow: "hidden" });
        gsap.set(cardsRef.current, { y: 50, opacity: 0 });

        const tl = gsap.timeline({ paused: true });

        tl.to(navEl, {
            height: calculateHeight,
            duration: 0.4,
            ease: "power3.out",
        });

        tl.to(
            cardsRef.current,
            { y: 0, opacity: 1, duration: 0.4, ease: "power3.out", stagger: 0.08 },
            "-=0.1"
        );

        return tl;
    }, [calculateHeight]);

    useLayoutEffect(() => {
        const tl = createTimeline();
        tlRef.current = tl;

        return () => {
            tl?.kill();
            tlRef.current = null;
        };
    }, [createTimeline]);

    useLayoutEffect(() => {
        const handleResize = () => {
            if (!tlRef.current) return;

            if (isExpanded) {
                const newHeight = calculateHeight();
                gsap.set(navRef.current, { height: newHeight });

                tlRef.current.kill();
                const newTl = createTimeline();
                if (newTl) {
                    newTl.progress(1);
                    tlRef.current = newTl;
                }
            } else {
                tlRef.current.kill();
                const newTl = createTimeline();
                if (newTl) {
                    tlRef.current = newTl;
                }
            }
        };

        window.addEventListener("resize", handleResize);
        return () => window.removeEventListener("resize", handleResize);
    }, [isExpanded, calculateHeight, createTimeline]);

    const toggleMenu = () => {
        const tl = tlRef.current;
        if (!tl) return;
        if (!isExpanded) {
            setIsHamburgerOpen(true);
            setIsExpanded(true);
            tl.play(0);
        } else {
            setIsHamburgerOpen(false);
            tl.eventCallback("onReverseComplete", () => setIsExpanded(false));
            tl.reverse();
        }
    };

    const setCardRef = (i: number) => (el: HTMLDivElement | null) => {
        if (el) cardsRef.current[i] = el;
    };

    return (
        <div className="absolute left-1/2 -translate-x-1/2 w-[90%] max-w-[800px] z-[99] top-[1.2em] md:top-[2em]">
            <nav
                ref={navRef}
                className={`block h-[60px] p-0 rounded-xl shadow-md relative overflow-hidden will-change-[height] bg-cream`}
            >
                {/* Top bar */}
                <div className="absolute inset-x-0 top-0 h-[60px] flex items-center justify-between p-2 pl-[1.1rem] z-[2]">
                    {/* Hamburger */}
                    <div
                        className={`group h-full flex flex-col items-center justify-center cursor-pointer gap-[6px] order-2 md:order-none`}
                        onClick={toggleMenu}
                        role="button"
                        aria-label={isExpanded ? "Close menu" : "Open menu"}
                        tabIndex={0}
                    >
                        <div
                            className={`w-[30px] h-[2px] bg-charcoal transition-[transform,opacity,margin] duration-300 ease-linear origin-center ${isHamburgerOpen ? "translate-y-[4px] rotate-45" : ""
                                } group-hover:opacity-75`}
                        />
                        <div
                            className={`w-[30px] h-[2px] bg-charcoal transition-[transform,opacity,margin] duration-300 ease-linear origin-center ${isHamburgerOpen ? "-translate-y-[4px] -rotate-45" : ""
                                } group-hover:opacity-75`}
                        />
                    </div>

                    {/* Logo */}
                    <div className="flex items-center gap-2 md:absolute md:left-1/2 md:top-1/2 md:-translate-x-1/2 md:-translate-y-1/2 order-1 md:order-none">
                        <div className="w-7 h-7 rounded-full bg-gradient-to-br from-terracotta to-beige flex items-center justify-center">
                            <span className="text-[10px] text-white font-bold">M</span>
                        </div>
                        <span className="text-lg font-semibold text-charcoal tracking-tight">
                            Memora
                        </span>
                    </div>

                    {/* CTA */}
                    <button
                        type="button"
                        className="hidden md:inline-flex border-0 rounded-[calc(0.75rem-0.2rem)] px-4 items-center h-full font-medium cursor-pointer transition-colors duration-300 bg-charcoal text-cream text-sm hover:bg-terracotta"
                    >
                        Get Started
                    </button>
                </div>

                {/* Expandable cards */}
                <div
                    className={`card-nav-content absolute left-0 right-0 top-[60px] bottom-0 p-2 flex flex-col items-stretch gap-2 justify-start z-[1] ${isExpanded
                        ? "visible pointer-events-auto"
                        : "invisible pointer-events-none"
                        } md:flex-row md:items-end md:gap-[12px]`}
                    aria-hidden={!isExpanded}
                >
                    {items.map((item, idx) => (
                        <div
                            key={`${item.label}-${idx}`}
                            className="select-none relative flex flex-col gap-2 p-[12px_16px] rounded-[calc(0.75rem-0.2rem)] min-w-0 flex-[1_1_auto] h-auto min-h-[60px] md:h-full md:min-h-0 md:flex-[1_1_0%]"
                            ref={setCardRef(idx)}
                            style={{
                                backgroundColor: item.bgColor,
                                color: item.textColor,
                            }}
                        >
                            <div className="font-normal tracking-[-0.5px] text-[18px] md:text-[22px]">
                                {item.label}
                            </div>
                            <div className="mt-auto flex flex-col gap-[2px]">
                                {item.links.map((lnk, i) => (
                                    <a
                                        key={`${lnk.label}-${i}`}
                                        className="inline-flex items-center gap-[6px] no-underline cursor-pointer transition-opacity duration-300 hover:opacity-75 text-[15px] md:text-[16px]"
                                        href={lnk.href}
                                        aria-label={lnk.ariaLabel}
                                        style={{ color: item.textColor }}
                                    >
                                        <GoArrowUpRight
                                            className="shrink-0"
                                            aria-hidden="true"
                                        />
                                        {lnk.label}
                                    </a>
                                ))}
                            </div>
                        </div>
                    ))}
                </div>
            </nav>
        </div>
    );
}
