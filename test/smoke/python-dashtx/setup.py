#!/usr/bin/env python

from setuptools import setup, find_packages
import os

from dashtx import __version__

requires = ['python-bitcointx>=1.0.0,<2']

setup(name='python-dashtx',
      version=__version__,
      description='Dash module for use with python-bitcointx',
      classifiers=[
          "Programming Language :: Python :: 3.6",
          "Programming Language :: Python :: 3.7",
          "Programming Language :: Python :: 3.8",
          "License :: OSI Approved :: GNU Lesser General Public License v3 or later (LGPLv3+)",
      ],
      python_requires='>=3.6',
      url='https://gitlab.com/thorchain/bifrost/python-dashtx',
      keywords='dash',
      packages=find_packages(),
      zip_safe=False,
      install_requires=requires,
      test_suite="dashtx.tests"
      )
