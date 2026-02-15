"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
    motion,
    useMotionValue,
    useTransform,
    type PanInfo,
    type MotionValue,
} from "framer-motion";
import React, { type JSX } from "react";

export interface CarouselItem {
    title: string;
    description: string;
    id: number;
    icon: React.ReactNode;
}

export interface CarouselProps {
    items?: CarouselItem[];
    baseWidth?: number;
    autoplay?: boolean;
    autoplayDelay?: number;
    pauseOnHover?: boolean;
    loop?: boolean;
    round?: boolean;
}

const DRAG_BUFFER = 0;
const VELOCITY_THRESHOLD = 500;
const GAP = 16;
const SPRING_OPTIONS = {
    type: "spring" as const,
    stiffness: 300,
    damping: 30,
};

interface CarouselCardProps {
    item: CarouselItem;
    index: number;
    itemWidth: number;
    round: boolean;
    trackItemOffset: number;
    x: MotionValue<number>;
    transition: Record<string, unknown>;
}

function CarouselCard({
    item,
    index,
    itemWidth,
    round,
    trackItemOffset,
    x,
    transition,
}: CarouselCardProps) {
    const range = [
        -(index + 1) * trackItemOffset,
        -index * trackItemOffset,
        -(index - 1) * trackItemOffset,
    ];
    const outputRange = [90, 0, -90];
    const rotateY = useTransform(x, range, outputRange, { clamp: false });

    return (
        <motion.div
            className={`relative shrink-0 flex flex-col ${round
                ? "items-center justify-center text-center bg-charcoal border-0"
                : "items-start justify-between bg-white border border-terracotta/20 rounded-[12px]"
                } overflow-hidden cursor-grab active:cursor-grabbing`}
            style={{
                width: itemWidth,
                height: round ? itemWidth : "100%",
                rotateY,
                ...(round && { borderRadius: "50%" }),
            }}
            transition={transition}
        >
            <div className={`${round ? "p-0 m-0" : "mb-4 p-5"}`}>
                <span className="flex h-[28px] w-[28px] items-center justify-center rounded-full bg-terracotta/10">
                    {item.icon}
                </span>
            </div>
            <div className="p-5">
                <div className="mb-1 font-black text-lg text-charcoal">{item.title}</div>
                <p className="text-sm text-charcoal/80">{item.description}</p>
            </div>
        </motion.div>
    );
}

export default function Carousel({
    items = [],
    baseWidth = 300,
    autoplay = false,
    autoplayDelay = 3000,
    pauseOnHover = false,
    loop = false,
    round = false,
}: CarouselProps): JSX.Element {
    const containerPadding = 16;
    const itemWidth = baseWidth - containerPadding * 2;
    const trackItemOffset = itemWidth + GAP;

    const itemsForRender = useMemo(() => {
        if (!loop) return items ?? [];
        if (!items || items.length === 0) return [];
        return [items[items.length - 1], ...items, items[0]];
    }, [items, loop]);

    const [position, setPosition] = useState<number>(loop ? 1 : 0);
    const x = useMotionValue(0);
    const [isHovered, setIsHovered] = useState<boolean>(false);
    const [isJumping, setIsJumping] = useState<boolean>(false);
    const [isAnimating, setIsAnimating] = useState<boolean>(false);

    const containerRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (pauseOnHover && containerRef.current) {
            const container = containerRef.current;
            const handleMouseEnter = () => setIsHovered(true);
            const handleMouseLeave = () => setIsHovered(false);
            container.addEventListener("mouseenter", handleMouseEnter);
            container.addEventListener("mouseleave", handleMouseLeave);
            return () => {
                container.removeEventListener("mouseenter", handleMouseEnter);
                container.removeEventListener("mouseleave", handleMouseLeave);
            };
        }
    }, [pauseOnHover]);

    useEffect(() => {
        if (!autoplay || itemsForRender.length <= 1) return undefined;
        if (pauseOnHover && isHovered) return undefined;

        const timer = setInterval(() => {
            setPosition((prev) =>
                Math.min(prev + 1, itemsForRender.length - 1)
            );
        }, autoplayDelay);

        return () => clearInterval(timer);
    }, [autoplay, autoplayDelay, isHovered, pauseOnHover, itemsForRender.length]);

    useEffect(() => {
        const startingPosition = loop ? 1 : 0;
        setPosition(startingPosition);
        x.set(-startingPosition * trackItemOffset);
    }, [items?.length, loop, trackItemOffset, x]);

    useEffect(() => {
        if (!loop && position > itemsForRender.length - 1) {
            setPosition(Math.max(0, itemsForRender.length - 1));
        }
    }, [itemsForRender.length, loop, position]);

    const effectiveTransition = isJumping
        ? { duration: 0 }
        : SPRING_OPTIONS;

    const handleAnimationStart = () => {
        setIsAnimating(true);
    };

    const handleAnimationComplete = () => {
        if (!loop || itemsForRender.length <= 1) {
            setIsAnimating(false);
            return;
        }
        const lastCloneIndex = itemsForRender.length - 1;

        if (position === lastCloneIndex) {
            setIsJumping(true);
            const target = 1;
            setPosition(target);
            x.set(-target * trackItemOffset);
            requestAnimationFrame(() => {
                setIsJumping(false);
                setIsAnimating(false);
            });
            return;
        }

        if (position === 0) {
            setIsJumping(true);
            const target = items ? items.length : 0;
            setPosition(target);
            x.set(-target * trackItemOffset);
            requestAnimationFrame(() => {
                setIsJumping(false);
                setIsAnimating(false);
            });
            return;
        }

        setIsAnimating(false);
    };

    const handleDragEnd = (
        _: MouseEvent | TouchEvent | PointerEvent,
        info: PanInfo
    ): void => {
        const { offset, velocity } = info;
        const direction =
            offset.x < -DRAG_BUFFER || velocity.x < -VELOCITY_THRESHOLD
                ? 1
                : offset.x > DRAG_BUFFER || velocity.x > VELOCITY_THRESHOLD
                    ? -1
                    : 0;

        if (direction === 0) return;

        setPosition((prev) => {
            const next = prev + direction;
            const max = itemsForRender.length - 1;
            return Math.max(0, Math.min(next, max));
        });
    };

    const dragProps = loop
        ? {}
        : {
            dragConstraints: {
                left: -trackItemOffset * Math.max(itemsForRender.length - 1, 0),
                right: 0,
            },
        };

    const safeItems = items ?? [];
    const activeIndex =
        safeItems.length === 0
            ? 0
            : loop
                ? (position - 1 + safeItems.length) % safeItems.length
                : Math.min(position, safeItems.length - 1);

    return (
        <div
            ref={containerRef}
            className={`relative overflow-hidden p-4 ${round
                ? "rounded-full border border-white"
                : "rounded-[24px] border border-terracotta/10"
                }`}
            style={{
                width: `${baseWidth}px`,
                ...(round && { height: `${baseWidth}px` }),
            }}
        >
            <motion.div
                className="flex"
                drag={isAnimating ? false : "x"}
                {...dragProps}
                style={{
                    width: itemWidth,
                    gap: `${GAP}px`,
                    perspective: 1000,
                    perspectiveOrigin: `${position * trackItemOffset + itemWidth / 2}px 50%`,
                    x,
                }}
                onDragEnd={handleDragEnd}
                animate={{ x: -(position * trackItemOffset) }}
                transition={effectiveTransition}
                onAnimationStart={handleAnimationStart}
                onAnimationComplete={handleAnimationComplete}
            >
                {itemsForRender.map((item, index) => (
                    <CarouselCard
                        key={`${item?.id ?? index}-${index}`}
                        item={item}
                        index={index}
                        itemWidth={itemWidth}
                        round={round}
                        trackItemOffset={trackItemOffset}
                        x={x}
                        transition={effectiveTransition}
                    />
                ))}
            </motion.div>

            {/* Dots */}
            <div
                className={`flex w-full justify-center ${round
                    ? "absolute z-20 bottom-12 left-1/2 -translate-x-1/2"
                    : ""
                    }`}
            >
                <div className="mt-4 flex w-[150px] justify-between px-8">
                    {safeItems.map((_, index) => (
                        <motion.div
                            key={index}
                            className={`h-2 w-2 rounded-full cursor-pointer transition-colors duration-150 ${activeIndex === index
                                ? round
                                    ? "bg-white"
                                    : "bg-charcoal"
                                : round
                                    ? "bg-charcoal/60"
                                    : "bg-charcoal/20"
                                }`}
                            animate={{ scale: activeIndex === index ? 1.2 : 1 }}
                            onClick={() => setPosition(loop ? index + 1 : index)}
                            transition={{ duration: 0.15 }}
                        />
                    ))}
                </div>
            </div>
        </div>
    );
}
