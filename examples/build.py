"""Build a handwritten example with this interpreter; optionally run its test."""

import argparse
from pathlib import Path
import shutil
import subprocess
import sys


EXAMPLES = Path(__file__).resolve().parent

# Run setuptools in a subprocess so its command-line parsing and state stay local.
BUILD_EXTENSION = r'''
from pathlib import Path
import sys
import pybind11
from setuptools import Extension, setup

source, native, output, temporary, package = sys.argv[1:]
sys.argv = sys.argv[:1]
setup(
    name=package,
    version="0.0.0",
    ext_modules=[Extension(
        name=package + "._native",
        sources=[source],
        include_dirs=[pybind11.get_include(), native],
        library_dirs=[native],
        libraries=["bridge"],
        language="c++",
        extra_compile_args=["-std=c++17"],
        extra_link_args=["-Wl,-rpath,@loader_path"] if sys.platform == "darwin" else [],
        runtime_library_dirs=["$ORIGIN"] if sys.platform.startswith("linux") else [],
    )],
    script_args=["build_ext", "--force", "--build-lib", output, "--build-temp", temporary],
)
'''


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("example", help="example directory name, e.g. 01_basic")
    parser.add_argument("--test", action="store_true", help="run test.py after building")
    args = parser.parse_args()

    case = EXAMPLES / args.example
    if case.parent != EXAMPLES or not (case / "expected").is_dir():
        parser.error("expected an example directory name containing expected/")
    if sys.platform == "darwin":
        library = "libbridge.dylib"
    elif sys.platform.startswith("linux"):
        library = "libbridge.so"
    else:
        parser.error("supported platforms: macOS and Linux")

    try:
        import pybind11
        import setuptools
    except ImportError:
        parser.error("this Python interpreter needs pybind11 and setuptools installed")

    expected = case / "expected"
    packages = [p for p in expected.iterdir() if p.is_dir() and (p / "__init__.py").is_file()]
    if len(packages) != 1:
        parser.error("expected/ must contain exactly one Python package with __init__.py")
    package = packages[0]
    for source in [expected / "go/main.go", expected / "bindings.cpp", package / "__init__.pyi"]:
        if not source.is_file():
            parser.error("missing source: " + str(source))

    build = EXAMPLES / ".build" / case.name
    native = build / "native"
    output = build / "python"
    temporary = build / "temp"
    for directory in [native, output, temporary]:
        directory.mkdir(parents=True, exist_ok=True)

    def run(command):
        subprocess.run([str(part) for part in command], cwd=EXAMPLES.parent, check=True)

    run(["go", "build", "-buildmode=c-shared", "-o", native / library, expected / "go"])
    if sys.platform == "darwin":
        run(["install_name_tool", "-id", "@rpath/" + library, native / library])

    # Reassemble the package to avoid retaining removed APIs or old extensions.
    destination = output / package.name
    if destination.exists():
        shutil.rmtree(destination)
    shutil.copytree(package, destination, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
    (destination / "py.typed").touch()
    shutil.copy2(native / library, destination / library)
    run([sys.executable, "-c", BUILD_EXTENSION, expected / "bindings.cpp", native,
         output, temporary, package.name])
    print("Built " + str(destination) + " with " + sys.executable, flush=True)
    if args.test:
        run([sys.executable, case / "test.py"])


if __name__ == "__main__":
    main()
