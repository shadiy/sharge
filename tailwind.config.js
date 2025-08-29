/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    // Configure Tailwind to scan your Go template files for class names
    // Adjust this path based on your project structure
    "./tmpl/**/*.html", 
    "./views/**/*.gohtml", 
  ],
  plugins: [
    // Add any Tailwind plugins you might be using, e.g., @tailwindcss/typography
  ],
};