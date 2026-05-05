import beakClosedImage from "@/assets/cialloclaw-pet/beak_closed.png";
import beakOpenImage from "@/assets/cialloclaw-pet/beak_open.png";
import bodyImage from "@/assets/cialloclaw-pet/body.png";
import bubbleAlertImage from "@/assets/cialloclaw-pet/bubble_alert.png";
import bubbleListeningImage from "@/assets/cialloclaw-pet/bubble_listening.png";
import bubbleSafeImage from "@/assets/cialloclaw-pet/bubble_safe.png";
import bubbleThinkingImage from "@/assets/cialloclaw-pet/bubble_thinking.png";
import eyeClosedImage from "@/assets/cialloclaw-pet/eye_closed.png";
import eyeOpenImage from "@/assets/cialloclaw-pet/eye_open.png";
import leftWingImage from "@/assets/cialloclaw-pet/left_wing.png";
import rightWingImage from "@/assets/cialloclaw-pet/right_wing.png";
import sparkleImage from "@/assets/cialloclaw-pet/sparkle_03.png";
import tailImage from "@/assets/cialloclaw-pet/tail.png";

export type FloatingPetMode = "idle" | "happy" | "alert" | "safe" | "listen" | "think";

export type FloatingPetLayerTransform = {
  opacity: number;
  position: {
    x: number;
    y: number;
  };
  rotation: number;
  scale: {
    x: number;
    y: number;
  };
};

export type FloatingPetBoneLayout = FloatingPetLayerTransform & {
  length: number;
};

export type FloatingPetCheekLayout = FloatingPetLayerTransform & {
  fill: string;
  fillOpacity: number;
  size: {
    h: number;
    w: number;
  };
};

export type FloatingPetFaceLayout = FloatingPetLayerTransform & {
  beak: FloatingPetLayerTransform & {
    beakClosed: FloatingPetLayerTransform;
    beakOpen: FloatingPetLayerTransform;
  };
  cheek: FloatingPetLayerTransform & {
    cheekLeft: FloatingPetCheekLayout;
    cheekRight: FloatingPetCheekLayout;
  };
  eyes: FloatingPetLayerTransform & {
    eyeClosedLeft: FloatingPetLayerTransform;
    eyeClosedRight: FloatingPetLayerTransform;
    eyeOpenLeft: FloatingPetLayerTransform;
    eyeOpenRight: FloatingPetLayerTransform;
  };
};

export type FloatingPetBubbleLayout = FloatingPetLayerTransform & {
  effects: {
    bubbleAlert: FloatingPetLayerTransform;
    bubbleListening: FloatingPetLayerTransform;
    bubbleSafe: FloatingPetLayerTransform;
    bubbleThinking: FloatingPetLayerTransform;
  };
};

export type FloatingPetRootBodyLayout = FloatingPetLayerTransform & {
  body: FloatingPetLayerTransform;
  cheek: FloatingPetFaceLayout["cheek"];
  eyes: FloatingPetFaceLayout["eyes"];
  beak: FloatingPetFaceLayout["beak"];
  face: FloatingPetLayerTransform;
  leftBone: FloatingPetBoneLayout;
  leftWing: FloatingPetLayerTransform;
  rightBone: FloatingPetBoneLayout;
  rightWing: FloatingPetLayerTransform;
  tail: FloatingPetLayerTransform;
  tailBone: FloatingPetBoneLayout;
};

export type FloatingPetInitialLayout = {
  bubble: FloatingPetBubbleLayout;
  rootBody: FloatingPetRootBodyLayout;
  sparkle: FloatingPetLayerTransform;
};

export const FLOATING_PET_STAGE_SIZE = 500;
export const FLOATING_PET_LOOP_DURATION_S = 2;
export const FLOATING_PET_QUICK_TAIL_DURATION_S = 1;
export const FLOATING_PET_EYE_BREATH_DURATION_S = 5;
export const FLOATING_PET_HAPPY_DURATION_MS = 2_000;
export const FLOATING_PET_HAPPY_END_DURATION_S = 0.5;
export const FLOATING_PET_EFFECT_END_DURATION_S = 10 / 12;

export const floatingPetAssets = {
  beakClosed: beakClosedImage,
  beakOpen: beakOpenImage,
  body: bodyImage,
  bubbleAlert: bubbleAlertImage,
  bubbleListening: bubbleListeningImage,
  bubbleSafe: bubbleSafeImage,
  bubbleThinking: bubbleThinkingImage,
  eyeClosed: eyeClosedImage,
  eyeOpen: eyeOpenImage,
  leftWing: leftWingImage,
  rightWing: rightWingImage,
  sparkle: sparkleImage,
  tail: tailImage,
} as const;

export type FloatingPetAssetName = keyof typeof floatingPetAssets;

export const floatingPetAssetDimensions: Record<FloatingPetAssetName, { width: number; height: number }> = {
  beakClosed: { width: 1254, height: 1254 },
  beakOpen: { width: 1254, height: 1254 },
  body: { width: 1254, height: 1254 },
  bubbleAlert: { width: 1254, height: 1254 },
  bubbleListening: { width: 1254, height: 1254 },
  bubbleSafe: { width: 1254, height: 1254 },
  bubbleThinking: { width: 1254, height: 1254 },
  eyeClosed: { width: 517, height: 517 },
  eyeOpen: { width: 453, height: 453 },
  leftWing: { width: 1254, height: 1254 },
  rightWing: { width: 1254, height: 1254 },
  sparkle: { width: 1254, height: 1254 },
  tail: { width: 1254, height: 1254 },
};

export const floatingPetInitialLayout: FloatingPetInitialLayout = {
  bubble: {
    opacity: 1,
    position: { x: 100, y: 152 },
    rotation: 0,
    scale: { x: 100, y: 100 },
    effects: {
      bubbleAlert: {
        opacity: 0,
        position: { x: -22, y: 6 },
        rotation: 0,
        scale: { x: 16.6, y: 16.6 },
      },
      bubbleListening: {
        opacity: 0,
        position: { x: -23.5, y: 5 },
        rotation: 0,
        scale: { x: 16.9, y: 16.9 },
      },
      bubbleSafe: {
        opacity: 0,
        position: { x: -23, y: 7.5 },
        rotation: 0,
        scale: { x: 12.3, y: 12 },
      },
      bubbleThinking: {
        opacity: 0,
        position: { x: 0, y: 0 },
        rotation: 0,
        scale: { x: 21.6, y: 21.6 },
      },
    },
  },
  sparkle: {
    opacity: 0,
    position: { x: 107.5, y: 165 },
    rotation: 0,
    scale: { x: 19, y: 19 },
  },
  rootBody: {
    opacity: 1,
    position: { x: 235.83, y: 248.94 },
    rotation: 0,
    scale: { x: 100, y: 100 },
    body: {
      opacity: 1,
      position: { x: -1.39, y: 0 },
      rotation: 0,
      scale: { x: 24.4, y: 24.4 },
    },
    face: {
      opacity: 1,
      position: { x: -26.31, y: -12.94 },
      rotation: 0,
      scale: { x: 100, y: 100 },
    },
    beak: {
      opacity: 1,
      position: { x: -0.03, y: 0 },
      rotation: 0,
      scale: { x: 100, y: 100 },
      beakClosed: {
        opacity: 1,
        position: { x: 0, y: 0 },
        rotation: 0,
        scale: { x: 9.7, y: 9.7 },
      },
      beakOpen: {
        opacity: 0,
        position: { x: 0, y: 0 },
        rotation: 0,
        scale: { x: 10.3, y: 10.3 },
      },
    },
    eyes: {
      opacity: 1,
      position: { x: 3.24, y: -9.5 },
      rotation: 0,
      scale: { x: 100, y: 100 },
      eyeClosedLeft: {
        opacity: 0,
        position: { x: -30, y: 2 },
        rotation: 0,
        scale: { x: 13, y: 13 },
      },
      eyeClosedRight: {
        opacity: 0,
        position: { x: 28, y: 2 },
        rotation: 0,
        scale: { x: 13, y: 13 },
      },
      eyeOpenLeft: {
        opacity: 1,
        position: { x: -29.76, y: 0.5 },
        rotation: 0,
        scale: { x: 14.9, y: 14.9 },
      },
      eyeOpenRight: {
        opacity: 1,
        position: { x: 28.24, y: 1.5 },
        rotation: 0,
        scale: { x: 14.9, y: 14.9 },
      },
    },
    cheek: {
      opacity: 1,
      position: { x: 0, y: 6.25 },
      rotation: 0,
      scale: { x: 100, y: 100 },
      cheekLeft: {
        fill: "#FACAC4",
        fillOpacity: 0.8,
        opacity: 1,
        position: { x: -60.53, y: -0.25 },
        rotation: 0,
        scale: { x: 88.5, y: 100 },
        size: { w: 29, h: 15.5 },
      },
      cheekRight: {
        fill: "#FACAC4",
        fillOpacity: 0.8,
        opacity: 1,
        position: { x: 60.97, y: -0.25 },
        rotation: 0,
        scale: { x: 85.4, y: 100 },
        size: { w: 29, h: 15.5 },
      },
    },
    leftBone: {
      length: 85.76,
      opacity: 1,
      position: { x: -86.73, y: -39.25 },
      rotation: 104.149,
      scale: { x: 100, y: 100 },
    },
    leftWing: {
      opacity: 1,
      position: { x: -93.89, y: 0.1 },
      rotation: -98.127,
      scale: { x: 10, y: 10 },
    },
    rightBone: {
      length: 111.26,
      opacity: 1,
      position: { x: 69.81, y: -41.35 },
      rotation: 63.113,
      scale: { x: 100, y: 100 },
    },
    rightWing: {
      opacity: 1,
      position: { x: 86.11, y: 9.5 },
      rotation: -1.31,
      scale: { x: 13.3, y: 13.3 },
    },
    tail: {
      opacity: 1,
      position: { x: 113.61, y: 77.6 },
      rotation: -36.046,
      scale: { x: 11, y: 11 },
    },
    tailBone: {
      length: 107.25,
      opacity: 1,
      position: { x: 56.53, y: 46.7 },
      rotation: 28.402,
      scale: { x: 100, y: 100 },
    },
  },
};
