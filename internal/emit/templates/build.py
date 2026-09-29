"""Build the generated Go library and Python extension with this interpreter."""

import json
from pathlib import Path
import shutil
import subprocess
import sys
import sysconfig


ROOT = Path(__file__).resolve().parent

# Run in a child interpreter so setuptools' global state and argv handling
# stay out of this script. Arguments are passed positionally.
BUILD_EXTENSION = r'''
import sys
import pybind11
from setuptools import Extension, setup

source, native, output, temporary, package, library = sys.argv[1:]
sys.argv = sys.argv[:1]
setup(
    name=package,
    version="0.0.0",
    ext_modules=[Extension(
        name=package + "._native",
        sources=[source],
        include_dirs=[pybind11.get_include(), native],
        library_dirs=[native],
        libraries=[library],
        language="c++",
        extra_compile_args=["-std=c++17"],
        extra_link_args=["-Wl,-rpath,@loader_path"] if sys.platform == "darwin" else [],
        runtime_library_dirs=["$ORIGIN"] if sys.platform.startswith("linux") else [],
    )],
    script_args=["build_ext", "--force", "--build-lib", output, "--build-temp", temporary],
)
'''


def main():
    if sys.platform == "darwin":
        suffix = ".dylib"
    elif sys.platform.startswith("linux"):
        suffix = ".so"
    else:
        raise SystemExit("supported platforms: macOS and Linux")
    if sysconfig.get_config_var("Py_GIL_DISABLED"):
        raise SystemExit("free-threaded Python is not supported")
    try:
        import pybind11  # noqa: F401
        import setuptools  # noqa: F401
    except ImportError:
        raise SystemExit("this interpreter needs pybind11 and setuptools")

    config = json.loads((ROOT / "bridge.json").read_text())
    package = config["package"]
    source_dir = Path(config["source_dir"])
    if not source_dir.is_dir():
        raise SystemExit("original Go source directory is missing: " + str(source_dir))
    library = "bridge_" + package
    filename = "lib" + library + suffix
    build = ROOT / ".build"
    native, output, temporary = build / "native", build / "python", build / "temp"
    for directory in [native, output, temporary]:
        directory.mkdir(parents=True, exist_ok=True)

    def run(command):
        subprocess.run([str(part) for part in command], cwd=source_dir, check=True)

    # Build named adapter files in the target module's context, so its module
    # requirements, workspace and local replacements remain authoritative.
    run(["go", "build", "-buildmode=c-shared", "-o", native / filename,
         *sorted((ROOT / "go").glob("*.go"))])
    if sys.platform == "darwin":
        run(["install_name_tool", "-id", "@rpath/" + filename, native / filename])
    destination = output / package
    if destination.exists():
        shutil.rmtree(destination)
    shutil.copytree(ROOT / package, destination,
                    ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
    # The extension finds the library beside itself through its rpath
    # (@loader_path on macOS, $ORIGIN on Linux).
    shutil.copy2(native / filename, destination / filename)
    run([sys.executable, "-c", BUILD_EXTENSION, ROOT / "bindings.cpp", native,
         output, temporary, package, library])
    print("Built " + str(destination) + " with " + sys.executable, flush=True)


if __name__ == "__main__":
    main()
