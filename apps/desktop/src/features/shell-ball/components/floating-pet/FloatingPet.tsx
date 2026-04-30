import type { CSSProperties } from "react";
import { cn } from "@/utils/cn";
import styles from "./FloatingPet.module.css";
import { floatingPetBaseLayers, floatingPetFaceAssets } from "./petAssets";

export type FloatingPetProps = {
  className?: string;
  size?: number | string;
  eyesClosed?: boolean;
};

/**
 * Renders the static floating-pet bird from aligned PNG layers that already
 * share the same transparent canvas.
 *
 * @param props Component size, custom className, and eye-state toggle.
 * @returns The stacked floating-pet artwork.
 */
export function FloatingPet({ className, size = "100%", eyesClosed = false }: FloatingPetProps) {
  const rootStyle: CSSProperties = {
    height: size,
    width: size,
  };
  const faceAsset = eyesClosed ? floatingPetFaceAssets.closed : floatingPetFaceAssets.open;

  return (
    <div className={cn(styles.root, className)} style={rootStyle}>
      {floatingPetBaseLayers.map((layer) => (
        <img
          key={layer.name}
          alt=""
          aria-hidden="true"
          className={styles.layer}
          draggable={false}
          src={layer.src}
        />
      ))}
      <img alt="" aria-hidden="true" className={styles.layer} draggable={false} src={faceAsset} />
    </div>
  );
}
