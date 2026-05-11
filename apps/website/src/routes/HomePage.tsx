import { motion } from "motion/react";
import { useEffect, useMemo, useState } from "react";
import Antigravity from "@/components/Antigravity";
import CircularGallery from "@/components/CircularGallery";
import Galaxy from "@/components/Galaxy";
import mascotFallback from "../../../desktop/src/assets/cialloclaw-pet/body.png";
import { HeroActionButton } from "@/components/HeroActionButton";

export function HomePage() {
  const [heroImage, setHeroImage] = useState("/assets/images/final.png");
  const [isDarkTheme, setIsDarkTheme] = useState(false);
  const galleryItems = useMemo(
    () => [
      { image: "/assets/images/floating-ball.png", text: "桌面协作" },
      { image: "/assets/images/dashboard.png", text: "语音承接" },
      { image: "/assets/images/dashboard-task.png", text: "文档入口" },
      { image: "/assets/images/dashboard-note.png", text: "正式交付" },
    ],
    [],
  );
  const ambientParticles = useMemo(
    () => [
      { left: "8%", top: "18%", size: 8 },
      { left: "16%", top: "68%", size: 6 },
      { left: "58%", top: "12%", size: 5 },
      { left: "78%", top: "22%", size: 7 },
      { left: "86%", top: "62%", size: 9 },
    ],
    [],
  );
  const heroActions = useMemo(
    () => [
      { label: "下载", href: "/docs/overview/versions" },
      { label: "使用教程", href: "/docs/overview" },
    ],
    [],
  );

  useEffect(() => {
    const rootElement = document.documentElement;

    const syncTheme = () => {
      setIsDarkTheme(rootElement.dataset.theme === "dark");
    };

    syncTheme();

    const observer = new MutationObserver(syncTheme);
    observer.observe(rootElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    return () => observer.disconnect();
  }, []);

  return (
    <>
      <section className="relative isolate flex h-full items-center justify-center overflow-hidden">
        <div className="absolute inset-0">
          {!isDarkTheme ? (
            <div className="absolute inset-0 pointer-events-none">
              <Antigravity
                count={300}
                magnetRadius={6}
                ringRadius={7}
                waveSpeed={0.4}
                waveAmplitude={1}
                particleSize={1.5}
                lerpSpeed={0.05}
                color="#6aeef3"
                autoAnimate
                particleVariance={1}
                rotationSpeed={0}
                depthFactor={1}
                pulseSpeed={3}
                particleShape="sphere"
                fieldStrength={10}
              />
            </div>
          ) : null}

          {isDarkTheme ? (
            <div className="absolute inset-0 pointer-events-auto">
              <Galaxy
                mouseRepulsion={false}
                mouseInteraction
                density={1}
                glowIntensity={0.3}
                saturation={0.5}
                hueShift={140}
                twinkleIntensity={0.3}
                rotationSpeed={0.1}
                repulsionStrength={3}
                autoCenterRepulsion={0}
                starSpeed={0.1}
                speed={1}
              />
            </div>
          ) : null}
          {ambientParticles.map((particle) => (
            <motion.span
              key={`${particle.left}-${particle.top}`}
              className="pointer-events-none absolute rounded-full blur-[1px]"
              style={{
                left: particle.left,
                top: particle.top,
                width: particle.size,
                height: particle.size,
                background: isDarkTheme ? "rgba(255,255,255,0.16)" : "rgba(0,0,0,0.10)",
              }}
              animate={{ opacity: [0.16, 0.46, 0.16], y: [0, -10, 0] }}
              transition={{ duration: 8 + particle.size / 2, repeat: Number.POSITIVE_INFINITY, ease: "easeInOut" }}
            />
          ))}
        </div>

        <motion.div
          initial={{ opacity: 0, y: 28 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.9, delay: 0.08, ease: [0.22, 1, 0.36, 1] }}
          className="pointer-events-none absolute inset-0 z-[1] h-screen w-screen overflow-hidden"
        >
          <div className="pointer-events-auto absolute inset-x-0 top-[12%] h-[460px] w-screen overflow-hidden">
            <CircularGallery
              items={galleryItems}
              bend={2}
              borderRadius={0.1}
              scrollSpeed={1.5}
              scrollEase={0.1}
              font="700 58px var(--cc-font-ui)"
            />
          </div>
        </motion.div>

        <div className="relative z-20 flex max-w-4xl -translate-y-[7rem] flex-col items-center px-4 text-center">
          <motion.h1
            initial={{ opacity: 0, y: 28 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.74, ease: [0.22, 1, 0.36, 1] }}
            className="cc-home-hero-title font-display text-[6rem] leading-[0.98] tracking-[-0.05em] text-[color:var(--cc-ink)]"
          >
            CialloClaw
          </motion.h1>
          <motion.p
            initial={{ opacity: 0, y: 22 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.74, delay: 0.12, ease: [0.22, 1, 0.36, 1] }}
            className="cc-home-hero-subtitle mt-4 max-w-[760px] text-[2rem] leading-[1.6] text-[color:var(--cc-ink-soft)]"
          >
            桌面悬浮球 Agent，你的专属桌宠助理
          </motion.p>
          <motion.div
            initial={{ opacity: 0, y: 18 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.74, delay: 0.24, ease: [0.22, 1, 0.36, 1] }}
            className="mt-8 flex items-center justify-center gap-12"
          >
            {heroActions.map((action) => (
              <HeroActionButton key={action.href} label={action.label} href={action.href} />
            ))}
          </motion.div>
        </div>

        <div className="pointer-events-none absolute bottom-[-20%] left-[50vw] z-10 -translate-x-1/2 select-none">
          <motion.img
            initial={{ opacity: 0, y: 86 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 1.02, delay: 0.18, ease: [0.22, 1, 0.36, 1] }}
            src={heroImage}
            alt="CialloClaw mascot"
            onError={() => setHeroImage(mascotFallback)}
            className="w-[1450px] max-w-none"
          />
        </div>

      </section>
    </>
  );
}
