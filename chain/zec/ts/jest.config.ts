import type { Config } from "jest";

const config: Config = {
  preset: "ts-jest",
  testEnvironment: "node",
  roots: ["<rootDir>/src"],
  testMatch: ["**/__tests__/**/*.ts", "**/?(*.)+(spec|test).ts"],
  transform: {
    "^.+\\.ts$": "ts-jest",
  },
  collectCoverageFrom: ["src/**/*.ts", "!src/**/*.d.ts", "!src/test.ts"],
  setupFilesAfterEnv: ["<rootDir>/src/setupTests.ts"],
  testTimeout: 30000, // 30 seconds for tests that might need network access
};

export default config;
