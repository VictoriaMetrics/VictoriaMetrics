import { execFileSync } from "child_process";
import { readdirSync, statSync } from "fs";
import { join } from "path";

// Function to find all test files recursively
function findTestFiles(dir) {
  let results = [];
  const list = readdirSync(dir);

  list.forEach((file) => {
    const filePath = join(dir, file);
    const stat = statSync(filePath);

    if (stat.isDirectory()) {
      // Recursively search directories
      results = results.concat(findTestFiles(filePath));
    } else if (file.endsWith(".test.ts") || file.endsWith(".test.tsx")) {
      // Found a test file
      results.push(filePath);
    }
  });

  return results;
}

// Find all test files in the src directory
const testFiles = findTestFiles("./src");
console.log(`Found ${testFiles.length} test files:\n`);
testFiles.forEach(file => console.log(`- ${file}`));
console.log("\nRunning tests...\n");

let failedTests = 0;
testFiles.forEach((file) => {
  console.log(`\n=== Running ${file} ===\n`);
  try {
    execFileSync("npx", ["tsx", file], { stdio: "inherit" });
  } catch (error) {
    failedTests++;
    console.error(`\n${file} failed\n`);
  }
});

// Summary
if (failedTests === 0) {
  console.log("\n✅ All tests passed!\n");
  process.exit(0);
} else {
  console.log(`\n❌ ${failedTests} test files failed.\n`);
  process.exit(1);
}
