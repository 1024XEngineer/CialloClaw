import bodyImage from "@/assets/floating-ball/body.png";
import feetImage from "@/assets/floating-ball/feet.png";
import faceClosedImage from "@/assets/floating-ball/face_closed.png";
import faceOpenImage from "@/assets/floating-ball/face_open.png";
import leftWingImage from "@/assets/floating-ball/left_wing.png";
import rightWingImage from "@/assets/floating-ball/right_wing.png";
import tailImage from "@/assets/floating-ball/tail.png";

export type FloatingPetLayerName = "tail" | "left_wing" | "right_wing" | "body" | "feet";

type FloatingPetLayer = {
  name: FloatingPetLayerName;
  src: string;
};

/**
 * These base layers share the same canvas and are intentionally rendered as a
 * simple full-frame stack instead of individually positioned sprites.
 */
export const floatingPetBaseLayers: ReadonlyArray<FloatingPetLayer> = [
  { name: "tail", src: tailImage },
  { name: "left_wing", src: leftWingImage },
  { name: "right_wing", src: rightWingImage },
  { name: "body", src: bodyImage },
  { name: "feet", src: feetImage },
];

export const floatingPetFaceAssets = {
  closed: faceClosedImage,
  open: faceOpenImage,
} as const;
